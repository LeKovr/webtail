package webtail

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed html
var content embed.FS

// FileServer return embedded or given fs
func FileServer(path string) http.Handler {
	if path != "" {
		return http.FileServer(http.Dir(path))
	}
	fsub, err := fs.Sub(content, "html")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fsub))
}
