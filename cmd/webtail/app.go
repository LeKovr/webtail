package main

import (
	"log"
	"net/http"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	stats_api "github.com/fukata/golang-stats-api-handler"

	"github.com/LeKovr/webtail"
)

// Config holds all config vars
type Config struct {
	Flags
	Tail webtail.Config `group:"Webtail Options"`
}

// run app and exit via given exitFunc
func run(exitFunc func(code int)) {

	var err error
	var cfg *Config
	defer func() { shutdown(exitFunc, err) }()
	cfg, err = setupConfig()
	if err != nil || cfg.Version {
		return
	}
	lg := setupLog(cfg.LogLevel == "debug" || cfg.Debug)

	var wt *webtail.Service
	wt, err = webtail.New(lg, cfg.Tail)
	if err != nil {
		return
	}
	go wt.Run()

	if cfg.HTML != "" {
		http.Handle("/", http.FileServer(http.Dir(cfg.HTML)))
	} else {
		http.Handle("/", http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo}))
	}
	http.HandleFunc("/tail", func(w http.ResponseWriter, r *http.Request) {
		wt.Handle(w, r)
	})
	http.HandleFunc("/api/stats", stats_api.Handler)
	lg.Print("Listen: ", cfg.Listen)
	err = http.ListenAndServe(cfg.Listen, nil)
}

// exit after deferred cleanups have run
func shutdown(exitFunc func(code int), e error) {
	if e != nil {
		var code int
		switch e {
		case ErrGotHelp:
			code = 3
		case ErrBadArgs:
			code = 2
		default:
			code = 1
			log.Printf("Run error: %+v", e)
		}
		exitFunc(code)
	}
}
