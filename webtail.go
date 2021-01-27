// Package webtail holds tailer service
// You don't need anything except Service methods
package webtail

import (
	"net/http"

	"github.com/go-logr/logr"
)

// codebeat:disable[TOO_MANY_IVARS]

// Config defines local application flags
type Config struct {
	Root        string `long:"root"  default:"log/"  description:"Root directory for log files"`
	Bytes       int64  `long:"bytes" default:"5000"  description:"tail from the last Nth location"`
	Lines       int    `long:"lines" default:"100"   description:"keep N old lines for new consumers"`
	MaxLineSize int    `long:"split" default:"180"   description:"split line if longer"`
	ListCache   int    `long:"cache" default:"2"      description:"Time to cache file listing (sec)"`
	Poll        bool   `long:"poll"  description:"use polling, instead of inotify"`
	Trace       bool   `long:"trace" description:"trace worker channels"`

	ClientBufferSize  int `long:"out_buf"      default:"256"  description:"Client Buffer Size"`
	WSReadBufferSize  int `long:"ws_read_buf"  default:"1024" description:"WS Read Buffer Size"`
	WSWriteBufferSize int `long:"ws_write_buf" default:"1024" description:"WS Write Buffer Size"`
}

// codebeat:enable[TOO_MANY_IVARS]

// Service holds WebTail service
type Service struct {
	cfg *Config
	hub *ClientHub
	log logr.Logger
}

// New creates WebTail service
func New(log logr.Logger, cfg *Config) (*Service, error) {
	tail, err := NewTailHub(log, cfg)
	if err != nil {
		return nil, err
	}
	hub := NewClientHub(log, tail)
	return &Service{cfg: cfg, hub: hub, log: log}, nil
}

// Run runs a message hub
func (wt *Service) Run() {
	wt.hub.Run()
}

// Close stops a message hub
func (wt *Service) Close() {
	wt.log.Info("Service Exiting")
	wt.hub.Close()
}

// Handle handles websocket requests from the peer
func (wt *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsUpgrader := upgrader(wt.cfg.WSReadBufferSize, wt.cfg.WSWriteBufferSize)
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		wt.log.Error(err, "Upgrade connection")
		return
	}
	client := &Client{
		hub:  wt.hub,
		conn: conn,
		send: make(chan []byte, wt.cfg.ClientBufferSize),
		log:  wt.log,
	}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.runWritePump()
	go client.runReadPump()
}
