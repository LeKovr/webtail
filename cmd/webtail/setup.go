package main

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/mattn/go-colorable"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jessevdk/go-flags"
)

// -----------------------------------------------------------------------------

// Flags defines local application flags
type Flags struct {
	Listen   string `long:"listen"      default:":8080"   description:"Http listen address"`
	HTML     string `long:"html"        default:""        description:"Serve pages from this path"`
	LogLevel string `long:"log_level"   default:"info"    description:"Log level [info|debug] (deprecated, see --debug)"`
	Version  bool   `long:"version"                       description:"Show version and exit"`
	Debug    bool   `long:"debug"                         description:"Show debug data"`
}

var (
	// ErrGotHelp returned after showing requested help
	ErrGotHelp = errors.New("help printed")
	// ErrBadArgs returned after showing command args error message
	ErrBadArgs = errors.New("option error printed")
)

// SetupConfig loads flags from args (if given) or command flags and ENV otherwise
func SetupConfig(args ...string) (*Config, error) {
	cfg := &Config{}
	p := flags.NewParser(cfg, flags.Default) //  HelpFlag | PrintErrors | PassDoubleDash

	var err error
	if len(args) == 0 {
		_, err = p.Parse()
	} else {
		_, err = p.ParseArgs(args)
	}
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil, ErrGotHelp
		}
		return nil, ErrBadArgs
	}
	return cfg, nil
}

// SetupLog creates logger
func SetupLog(withDebug bool) logr.Logger {
	aa := zap.NewDevelopmentEncoderConfig()
	aa.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapLog := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(aa),
		zapcore.AddSync(colorable.NewColorableStdout()),
		zapcore.DebugLevel,
	),
		zap.AddCaller(),
	)
	log := zapr.NewLogger(zapLog)

	if withDebug {
		log = log.V(1)
	}
	return log
}
