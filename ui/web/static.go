package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var assets embed.FS

// staticHandler serves the browser UI from the binary itself, so `adraw serve`
// has nothing to deploy alongside it.
//
// Caching is disabled. Embedded files carry no modification time, so a browser
// has no validator to revalidate against and falls back to heuristic caching —
// which means an edited app.js keeps serving the stale copy until a manual hard
// reload. The assets are local and a few kilobytes; there is nothing to gain by
// caching them and a confusing failure mode to avoid.
func staticHandler() http.Handler {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic("web: embedded assets missing: " + err.Error())
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
}
