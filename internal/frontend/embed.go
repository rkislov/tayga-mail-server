// Package frontend embeds the built Web UI (placeholder until UI is implemented).
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
)

// Dist holds static assets under frontend/dist.
//
//go:embed dist/*
var Dist embed.FS

// Handler serves embedded static files at /.
func Handler() http.Handler {
	sub, err := fs.Sub(Dist, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
