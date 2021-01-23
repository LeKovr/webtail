package internal

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFS(t *testing.T) {
	fs := FS()
	_, ok := fs.(http.FileSystem)
	assert.True(t, ok)
}
