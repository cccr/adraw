// Package render defines the renderer layer: the interface that turns a
// drawing.Frame into a concrete output format, and the registry renderers
// register themselves with.
package render

import (
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/cccr/adraw/pkg/drawing"
)

// Renderer converts a Frame into one output format. io.Writer covers writing
// to a file, to stdout, or over a network with no further abstraction.
type Renderer interface {
	// Name is the stable identifier used by the CLI and registry.
	Name() string

	// Ext is the file extension this renderer produces, without a dot. The CLI
	// uses it to pick a renderer from an output filename.
	Ext() string

	Render(w io.Writer, f drawing.Frame) error
}

var (
	mu       sync.RWMutex
	registry = map[string]Renderer{}
)

// Register adds r to the registry. Renderers call this from init(). It panics
// on a duplicate or empty name.
func Register(r Renderer) {
	mu.Lock()
	defer mu.Unlock()
	name := r.Name()
	if name == "" {
		panic("render: Register called with empty name")
	}
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("render: duplicate registration of %q", name))
	}
	registry[name] = r
}

// Get returns the registered renderer with the given name.
func Get(name string) (Renderer, error) {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("render: no renderer named %q (have %v)", name, namesLocked())
	}
	return r, nil
}

// ByExt returns the renderer producing the given file extension, with or
// without a leading dot. This is how the CLI infers the renderer from --out.
func ByExt(ext string) (Renderer, error) {
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range registry {
		if r.Ext() == ext {
			return r, nil
		}
	}
	return nil, fmt.Errorf("render: no renderer produces %q files (have %v)", ext, namesLocked())
}

// Names lists every registered renderer, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return namesLocked()
}

func namesLocked() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
