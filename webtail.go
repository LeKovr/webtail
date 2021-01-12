package webtail

import (
	"github.com/LeKovr/go-base/log"
	"github.com/LeKovr/webtail/tailer"
)

type Config struct {
	tailer.Config
}

type Service struct {
	hub *Hub
	lg  log.Logger
}

func New(lg log.Logger, cfg Config) (*Service, error) {

	tail, err := tailer.New(lg, cfg.Config)
	if err != nil {
		return nil, err
	}
	hub := newHub(lg, tail)
	return &Service{hub: hub, lg: lg}, nil
}

func (wt *Service) Run() {
	wt.hub.run()
}
