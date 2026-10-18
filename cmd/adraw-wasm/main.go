//go:build js && wasm

// Command adraw-wasm runs the web API inside the browser, so the UI can be
// hosted as static files with no server behind it.
//
// It does not reimplement the API. Each call is turned into an http.Request
// and served by the very same web.Handler that `adraw serve` uses, so the
// browser receives byte-identical responses — same JSON, same errors, same
// exported files — and there is no second copy of the API to drift.
//
// The page talks to it through wasm.js, which routes the UI's fetch('/api/…')
// calls here. app.js does not know which of the two it is running against.
package main

import (
	"net/http/httptest"
	"syscall/js"

	"github.com/cccr/adraw/ui/web"

	_ "github.com/cccr/adraw/patches"
	_ "github.com/cccr/adraw/renderers"
)

func main() {
	handler := web.Handler()

	// adrawServe(url) → {status, headers, body}. Synchronous: generation takes
	// milliseconds and touches nothing asynchronous, so there is nothing to
	// wait on. wasm.js wraps the result in a Response.
	js.Global().Set("adrawServe", js.FuncOf(func(this js.Value, args []js.Value) any {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", args[0].String(), nil))

		headers := map[string]any{}
		for k := range rec.Header() {
			headers[k] = rec.Header().Get(k)
		}
		return map[string]any{
			"status":  rec.Code,
			"headers": headers,
			"body":    rec.Body.String(),
		}
	}))

	// adrawConfig(text) supplies adraw.json. The browser fetches it as a
	// plain static file, so the gallery stays tunable without a rebuild.
	js.Global().Set("adrawConfig", js.FuncOf(func(this js.Value, args []js.Value) any {
		web.SetConfigData([]byte(args[0].String()))
		return nil
	}))

	js.Global().Call("adrawReady")
	select {} // keep the exported functions alive
}
