package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	stats_api "github.com/fukata/golang-stats-api-handler"

	"github.com/LeKovr/webtail"
//	"github.com/LeKovr/webtail/cmd/webtail/internal"
)

// Config holds all config vars
type Config struct {
	Flags
	Listen   string `long:"listen"      default:":8080"   description:"Http listen address"`
	HTML     string `long:"html"        default:""        description:"Serve pages from this path"`
	LogLevel string `long:"log_level"   default:"info"    description:"Log level [info|debug] (deprecated, see --debug)"`

	Tail webtail.Config `group:"Webtail Options"`
}

// Actual version value will be set at build time
var version = "0.0-dev"

// Run app and exit via given exitFunc
func Run(exitFunc func(code int)) {
	cfg, err := SetupConfig()
	log := SetupLog(err != nil || cfg.Debug)
	defer func() { Shutdown(exitFunc, err, log) }()
	log.Info("WebTail. Tail (log)files via web.", "v", version)
	if err != nil || cfg.Version {
		return
	}
	var wt *webtail.Service
	wt, err = webtail.New(log, &cfg.Tail)
	if err != nil {
		return
	}
	http.Handle("/", webtail.FileServer(cfg.HTML))
	http.Handle("/tail", wt)
	http.HandleFunc("/api/stats", stats_api.Handler)
	log.Info("Listen", "addr", cfg.Listen)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go wt.Run()
	go func() {
		// service connections
		if err = http.ListenAndServe(cfg.Listen, nil); err != nil && err != http.ErrServerClosed {
			quit <- os.Interrupt
		}
	}()
	<-quit
	wt.Close()
	log.Info("Server stopped")
}
