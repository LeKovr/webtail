//go:generate go-bindata -pkg $GOPACKAGE -prefix ../../html -o bindata.go ../../html/
package main

import (
	"fmt"
	"golang.org/x/net/websocket"
	"net/http"
	"os"
	"path"
	"runtime"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/jessevdk/go-flags"

	"github.com/LeKovr/go-base/log"
	"github.com/LeKovr/webtail/manager"
	"github.com/LeKovr/webtail/api"
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
	Tail manager.Config `group:"Tail Options"`
	API  api.Config     `group:"API Options"`
	Log  LogConfig      `group:"Logging Options"`
}

// -----------------------------------------------------------------------------

func main() {

	cfg, lg := setUp()
	lg.Printf("info: %s v %s. WebTail, tail logfiles via web", path.Base(os.Args[0]), Version)
	lg.Print("info: Copyright (C) 2016, Alexey Kovrizhkin <ak@elfire.ru>")

	_, err := os.Stat(cfg.Tail.Root)
	panicIfError(lg, err, "Logfile root dir")

	tm, err := manager.New(lg, cfg.Tail)
	panicIfError(lg, err, "Create tail manager")

	srv := api.Server{
		Config:  cfg.API,
		Root:    cfg.Tail.Root,
		Log:     lg,
		Manager: tm,
	}
	srv.Init()

	logs, err := srv.LoadLogs()
	panicIfError(lg, err, "Load logfile list")
	lg.Printf("info: Logfiles root %s contains %d item(s)", cfg.Tail.Root, len(*logs))

	http.Handle("/tail", websocket.Handler(srv.Handler))
	http.HandleFunc("/stats", srv.StatsHandler)
	if cfg.HTML != "" {
		http.Handle("/", http.FileServer(http.Dir(cfg.HTML)))
	} else {
		http.Handle("/", http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo}))
	}
	lg.Printf("info: Listen at %s", cfg.Listen)
	err = http.ListenAndServe(cfg.Listen, nil)
	panicIfError(lg, err, "Listen")
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
