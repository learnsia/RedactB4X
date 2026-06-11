package redactorpii

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
)

func init() {
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".html", "text/html; charset=utf-8")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
}

//go:embed static
var staticFS embed.FS

// StaticHandler returns an http.Handler that serves the embedded static files.
func StaticHandler() http.Handler {
	fsys, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return staticContentType(http.FileServer(http.FS(fsys)))
}

// staticContentType sets Content-Type from the file extension when serving embedded files.
func staticContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ext := filepath.Ext(r.URL.Path); ext != "" {
			if ct := mime.TypeByExtension(ext); ct != "" {
				w.Header().Set("Content-Type", ct)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// DashboardHTML returns the content of static/index.html for the root handler.
func DashboardHTML() string {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		return ""
	}
	return string(data)
}
