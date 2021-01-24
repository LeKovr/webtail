package webtail

import (
	"net/http"

	"github.com/LeKovr/go-base/log"
)

// Service holds WebTail service
type Service struct {
	hub *ClientHub
	lg  log.Logger
}

// New creates WebTail service
func New(lg log.Logger, cfg Config) (*Service, error) {

	tail, err := NewTailHub(lg, cfg)
	if err != nil {
		return nil, err
	}
	hub := newClientHub(lg, tail)
	return &Service{hub: hub, lg: lg}, nil
}

// Run runs a message hub
func (wt *Service) Run() {
	wt.hub.run()
}

// Handle handles websocket requests from the peer
func (wt *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wt.lg.Println(err)
		return
	}
	client := &Client{hub: wt.hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
