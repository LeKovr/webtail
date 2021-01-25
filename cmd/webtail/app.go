package main

import (
	"log"
	"net/http"

	stats_api "github.com/fukata/golang-stats-api-handler"

	"github.com/LeKovr/webtail"
	"github.com/LeKovr/webtail/cmd/webtail/internal"
)

// Config holds all config vars
type Config struct {
	Flags
	Tail webtail.Config `group:"Webtail Options"`
}

// Run app and exit via given exitFunc
func Run(exitFunc func(code int)) {
	var err error

	var cfg *Config

	defer func() { shutdown(exitFunc, err) }()
	cfg, err = SetupConfig()
	if err != nil || cfg.Version {
		return
	}
	lg := SetupLog(cfg.LogLevel == "debug" || cfg.Debug)

	var wt *webtail.Service
	wt, err = webtail.New(lg, &cfg.Tail)
	if err != nil {
		return
	}
	go wt.Run()

	http.Handle("/", FileServer(cfg.HTML))
	http.Handle("/tail", wt)
	http.HandleFunc("/api/stats", stats_api.Handler)
	lg.Print("Listen: ", cfg.Listen)
	err = http.ListenAndServe(cfg.Listen, nil)
}

// FileServer return embedded or given fs
func FileServer(path string) http.Handler {
	if path != "" {
		return http.FileServer(http.Dir(path))
	}
	return http.FileServer(internal.FS())
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
			log.Printf("Run error: %+v", e)
			code = 1
		}
		exitFunc(code)
	}
}
