package main

import (
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
