//go:generate go-bindata -pkg $GOPACKAGE -prefix ../../html -o bindata.go ../../html/

package main

import (
	"errors"
	baselog "log"
	"os"

	"github.com/LeKovr/go-base/log"
	"github.com/comail/colog"
	"github.com/jessevdk/go-flags"
)

// -----------------------------------------------------------------------------

// Flags defines local application flags
type Flags struct {
	Listen   string `long:"listen"      default:":8080"   description:"Http listen address"`
	HTML     string `long:"html"        default:""        description:"Serve pages from this path"`
	Version  bool   `long:"version"                       description:"Show version and exit"`
	LogLevel string `long:"log_level"   default:"info"    description:"Log level [info|debug] (deprecated, see --debug)"`
	Debug    bool   `long:"debug"       description:"Show debug data"`
}

var (
	// ErrGotHelp returned after showing requested help
	ErrGotHelp = errors.New("help printed")
	// ErrBadArgs returned after showing command args error message
	ErrBadArgs = errors.New("option error printed")
)

// setupConfig loads flags from args (if given) or command flags and ENV otherwise
func setupConfig(args ...string) (*Config, error) {
	cfg := &Config{}
	p := flags.NewParser(cfg, flags.Default) //  HelpFlag | PrintErrors | PassDoubleDash
	var err error
	if len(args) == 0 {
		_, err = p.Parse()
	} else {
		_, err = p.ParseArgs(args)
	}
	if err != nil {
		//fmt.Printf("Args error: %#v", err)
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil, ErrGotHelp
		}
		return nil, ErrBadArgs
	}
	return cfg, nil
}

func setupLog(withDebug bool) log.Logger {
	var ll colog.Level
	if withDebug {
		ll = colog.LDebug
	} else {
		ll = colog.LInfo
	}
	cl := colog.NewCoLog(os.Stderr, "", baselog.Lshortfile|baselog.Ldate|baselog.Ltime)
	cl.SetMinLevel(ll)
	cl.SetDefaultLevel(ll)
	lg := cl.NewLogger()
	return lg

}
