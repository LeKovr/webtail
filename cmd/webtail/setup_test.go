package main

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	// Save original args
	a := os.Args

	tests := []struct {
		name string
		code int
		args []string
	}{
		{"Help", 3, []string{"-h"}},
		{"UnknownFlag", 2, []string{"-0"}},
		{"UnknownRoot", 1, []string{"--root", "/notexists"}},
		{"UnknownPort", 1, []string{"--listen", ":xx", "--root", "/tmp"}},
	}
	for _, tt := range tests {
		os.Args = append([]string{a[0]}, tt.args...)
		var c int
		run(func(code int) { c = code })
		assert.Equal(t, tt.code, c, tt.name)
	}

	// Restore original args
	os.Args = a
}

func TestSetupConfig(t *testing.T) {
	cfg, err := setupConfig("--debug")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestFileServer(t *testing.T) {
	fs := fileserver("")
	assert.NotNil(t, fs)
	fs = fileserver("/tmp")
	assert.NotNil(t, fs)
}
func TestSetupLog(t *testing.T) {

	tests := []struct {
		name     string
		debug    bool
		wantRows int
	}{
		{"Debug", true, 2},
		{"NoDebug", false, 1},
	}
	for _, tt := range tests {
		l := setupLog(tt.debug)
		assert.NotNil(t, l)
	}

}

func TestShutdown(t *testing.T) {
	err := errors.New("unknown")
	var c int
	shutdown(func(code int) { c = code }, err)
	assert.Equal(t, 1, c)
}
