package main_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	cmd "github.com/LeKovr/webtail/cmd/webtail"
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
		{"UnknownRoot", 1, []string{"--debug", "--root", "/notexists"}},
		{"UnknownPort", 1, []string{"--debug", "--listen", ":xx", "--root", "./"}},
	}
	for _, tt := range tests {
		os.Args = append([]string{a[0]}, tt.args...)
		var c int
		cmd.Run(func(code int) { c = code })
		assert.Equal(t, tt.code, c, tt.name)
	}
	// Restore original args
	os.Args = a
}
