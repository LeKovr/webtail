package main

import (
	"os"

	"github.com/LeKovr/go-base/log"
	"github.com/comail/colog"
	baselog "log"
)

// LogConfig defines logger flags
type LogConfig struct {
	Level string `long:"log_level"   default:"info"           description:"Log level [warn|info|debug]"`
}

// NewLog creates new logger
func NewLog(cfg LogConfig) (log.Logger, error) {

	lvl, err := colog.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	cl := colog.NewCoLog(os.Stderr, "", baselog.Lshortfile|baselog.Ldate|baselog.Ltime)
	cl.SetMinLevel(lvl)
	cl.SetDefaultLevel(lvl)
	lg := cl.NewLogger()
	return lg, nil
}
