BIN := adraw

# CGO is off deliberately. Importing net/http pulls in the cgo-backed DNS
# resolver, which forces external linking, and Go 1.22's external linker does
# not emit the LC_UUID load command this macOS linker requires - the binary
# builds but aborts on launch. The internal linker has no such problem, and the
# pure-Go resolver is entirely adequate here.
export CGO_ENABLED = 0

.PHONY: build test vet check serve wasm serve-wasm deploy-klmn clean

build:
	go build -o $(BIN) ./cmd/adraw

serve: build
	./$(BIN) serve

# The UI as static files, with the engine compiled to WebAssembly in place of
# a server. Copy out/web/ anywhere that serves files - e.g. a Hugo site's
# static/adraw/ - and it runs entirely in the browser. wasm_exec.js must come
# from the same Go that built the module; Go 1.24 moved it from misc/wasm to
# lib/wasm, hence looking in both.
WASM_OUT := out/web

wasm:
	rm -rf $(WASM_OUT) && mkdir -p $(WASM_OUT)
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o $(WASM_OUT)/adraw.wasm ./cmd/adraw-wasm
	cp ui/web/static/app.js ui/web/static/style.css ui/web/wasm/wasm.js adraw.json $(WASM_OUT)/
	cp "$$(ls "$$(go env GOROOT)"/lib/wasm/wasm_exec.js "$$(go env GOROOT)"/misc/wasm/wasm_exec.js 2>/dev/null | head -1)" $(WASM_OUT)/
	sed 's|<script src="app.js"></script>|<script src="wasm_exec.js"></script><script src="wasm.js"></script><script src="app.js"></script>|' \
		ui/web/static/index.html > $(WASM_OUT)/index.html

serve-wasm: wasm
	cd $(WASM_OUT) && python3 -m http.server 8081

# Publish the engine to klmn.xyz/tools/adraw/. See scripts/deploy-klmn.sh.
deploy-klmn: check wasm
	./scripts/deploy-klmn.sh

test:
	go test ./...

# The composites check is disabled deliberately. It exists to catch field
# reordering in structs from other packages, but geom's Vec2 is a two-field
# geometric primitive from this same module, and writing Vec2{X: x, Y: y}
# throughout the vector and polygon maths costs far more in readability than
# the check could ever return. Every other vet analyser stays on.
vet:
	go vet -composites=false ./...

check: vet test

clean:
	rm -f $(BIN)
