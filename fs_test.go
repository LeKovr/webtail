package webtail

import (
	"testing"

	"github.com/stretchr/testify/assert"

)

func TestFileServer(t *testing.T) {
	fs := FileServer("")
	assert.NotNil(t, fs, "Embedded FS defined")
	fs = FileServer("html")
	assert.NotNil(t, fs, "Local FS defined")
}
