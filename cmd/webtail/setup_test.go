package main_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	cmd "github.com/LeKovr/webtail/cmd/webtail"
)

func TestSetupConfig(t *testing.T) {
	cfg, err := cmd.SetupConfig("--debug")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestSetupLog(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		wantRows []string
	}{
		{"Debug", true, []string{"debug", "info", "error"}},
		{"NoDebug", false, []string{"info", "error"}},
	}
	for _, tt := range tests {
		logRows := []string{}
		hook := func(e zapcore.Entry) error {
			logRows = append(logRows, e.Message)
			return nil
		}
		l := cmd.SetupLog(tt.debug, zap.Hooks(hook))
		l.V(1).Info("debug")
		l.Info("info")
		l.Error(nil, "error")
		assert.Equal(t, tt.wantRows, logRows)
	}
}

func TestShutdown(t *testing.T) {
	err := errors.New("unknown")
	logRows := []string{}
	hook := func(e zapcore.Entry) error {
		logRows = append(logRows, e.Message)
		return nil
	}
	l := cmd.SetupLog(false, zap.Hooks(hook))
	var c int

	cmd.Shutdown(func(code int) { c = code }, err, l)
	assert.Equal(t, 1, c)
	assert.Equal(t, []string{"Run error"}, logRows)
}
