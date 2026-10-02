// Package webapp embeds the minimal PWA (static HTML/CSS/vanilla JS, no build step) served at /app/.
package webapp

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
)

func init() {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
}

//go:embed static
var staticFS embed.FS

// Handler serves the PWA files.
func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
