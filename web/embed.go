// Package web embeds the frontend dashboard assets.
// The assets are built from frontend/ using Vite + Svelte.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Assets returns the embedded frontend assets filesystem.
// The returned FS has dist/ as its root, so files are accessed
// directly (e.g., "index.html" not "dist/index.html").
func Assets() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
