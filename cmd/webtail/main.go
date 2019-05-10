//go:generate go-bindata -pkg $GOPACKAGE -prefix ../../html -o bindata.go ../../html/

package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	stats_api "github.com/fukata/golang-stats-api-handler"

	"github.com/LeKovr/go-base/log"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/jessevdk/go-flags"

	"github.com/LeKovr/webtail/tailer"
)

// -----------------------------------------------------------------------------

// Flags defines local application flags
type Flags struct {
	Listen  string `long:"listen"      default:":8080"   description:"Http listen address"`
	HTML    string `long:"html"        default:""        description:"Serve pages from this path"`
	Version bool   `long:"version"                       description:"Show version and exit"`
}

// Config holds all config vars
type Config struct {
	Flags
	Tail tailer.Config `group:"Tail Options"`
	Log  LogConfig     `group:"Logging Options"`
}

// -----------------------------------------------------------------------------

func main() {
	cfg, lg := setUp()
	lg.Printf("info: webtail v%s. WebTail, tail logfiles via web", Version)
	lg.Print("info: Copyright (C) 2016-2017, Alexey Kovrizhkin <lekovr+webtail@gmail.com>")

	tail, err := tailer.New(lg, cfg.Tail)
	panicIfError(nil, err, "New tail")

	hub := newHub(lg, tail)
	go hub.run()

	if cfg.HTML != "" {
		http.Handle("/", http.FileServer(http.Dir(cfg.HTML)))
	} else {
		http.Handle("/", http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo}))
	}
	http.HandleFunc("/tail", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	http.HandleFunc("/api/stats", stats_api.Handler)
	lg.Print("Listen: ", cfg.Listen)
	err = http.ListenAndServe(cfg.Listen, nil)
	panicIfError(nil, err, "ListenAndServe")

}

// -----------------------------------------------------------------------------

func setUp() (cfg *Config, lg log.Logger) {
	cfg = &Config{}
	p := flags.NewParser(cfg, flags.Default)

	_, err := p.Parse()
	if err != nil {
		os.Exit(1) // error message written already
	}
	if cfg.Version {
		// show version & exit
		fmt.Printf("%s\n%s\n%s", Version, Build, Commit)
		os.Exit(0)
	}

	// use all CPU cores for maximum performance
	runtime.GOMAXPROCS(runtime.NumCPU())

	lg, err = NewLog(cfg.Log)
	panicIfError(nil, err, "Parse loglevel")
	return
}

// -----------------------------------------------------------------------------

func panicIfError(lg log.Logger, err error, msg string) {
	if err != nil {
		if lg != nil {
			lg.Printf("error: %s error: %s", msg, err.Error())
		} else {
			fmt.Printf("error: %s error: %s", msg, err.Error())
		}
		os.Exit(1)
	}
}
