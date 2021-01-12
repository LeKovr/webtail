package webtail

import (
	"github.com/LeKovr/go-base/log"
)

// Service holds WebTail service
type Service struct {
	hub *Hub
	lg  log.Logger
}

// New creates WebTail service
func New(lg log.Logger, cfg Config) (*Service, error) {

	tail, err := NewTailer(lg, cfg)
	if err != nil {
		return nil, err
	}
	hub := newHub(lg, tail)
	return &Service{hub: hub, lg: lg}, nil
}

// Run runs a message hub
func (wt *Service) Run() {
	wt.hub.run()
}
