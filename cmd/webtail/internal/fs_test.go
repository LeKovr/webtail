package internal_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LeKovr/webtail/cmd/webtail/internal"
)

func TestFS(t *testing.T) {
	fs := internal.FS()
	_, ok := fs.(http.FileSystem)
	assert.True(t, ok)
}
