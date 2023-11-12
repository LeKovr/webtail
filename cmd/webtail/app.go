package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	stats_api "github.com/fukata/golang-stats-api-handler"

	"github.com/LeKovr/go-kit/config"
	"github.com/LeKovr/go-kit/logger"
	"github.com/LeKovr/go-kit/ver"

	"github.com/LeKovr/webtail"
)

// Config holds all config vars
type Config struct {
	Listen   string `long:"listen"      default:":8080"   description:"Http listen address"`
	HTML     string `long:"html"        default:""        description:"Serve pages from this path"`

	Logger logger.Config  `group:"Logging Options" namespace:"log" env-namespace:"LOG"`
	Tail   webtail.Config `group:"Webtail Options"`
}

var (
	// App version, actual value will be set at build time.
	version = "0.0-dev"

	// Repository address, actual value will be set at build time.
	repo = "repo.git"
)

// Run app and exit via given exitFunc
func Run(exitFunc func(code int)) {
	// Load config
	var cfg Config
	err := config.Open(&cfg)
	defer func() { config.Close(err, exitFunc) }()
	if err != nil {
		return
	}
	log := logger.New(cfg.Logger, nil)
	log.Info("WebTail. Tail (log)files via web.", "v", version)

/*
	cfg, err := SetupConfig()
	log := SetupLog(err != nil || cfg.Debug)
	defer func() { Shutdown(exitFunc, err, log) }()
	if err != nil || cfg.Version {
		return
	}
*/
	var wt *webtail.Service
	wt, err = webtail.New(log, &cfg.Tail)
	if err != nil {
		return
	}
	go ver.Check(log, repo, version)
	http.Handle("/", webtail.FileServer(cfg.HTML))
	http.Handle("/tail", wt)
	http.HandleFunc("/api/stats", stats_api.Handler)
	log.Info("Listen", "addr", cfg.Listen)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go wt.Run()
	go func() {
		// service connections
		s := &http.Server{
			Addr:           cfg.Listen,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		if err = s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			quit <- os.Interrupt
		}
	}()
	<-quit
	wt.Close()
	log.Info("Server stopped")
}
