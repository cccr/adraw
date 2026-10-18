// Package patch defines the engine layer: the Patch interface that every
// generator implements, the parameter description mechanism that lets any UI
// drive any patch, and the registry patches register themselves with.
package patch

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cccr/adraw/pkg/drawing"
)

// Patch is one generator — an image, a style, an algorithm.
type Patch interface {
	// Name is the stable identifier used by the CLI and registry.
	Name() string

	// Description is one or two sentences on what this patch draws, shown by
	// any UI that lists patches.
	Description() string

	// Params describes every control this patch exposes. UIs are built from
	// this list alone.
	//
	// This is why patches describe themselves rather than being described by a
	// config file: a config would be a second copy of the same truth, free to
	// drift, and nothing could check that it lists the parameters the patch
	// actually reads. Here the patch and its controls cannot disagree.
	Params() []ParamSpec

	// Generate produces one complete drawing. It must be deterministic: the
	// same Request always yields the same Frame.
	Generate(ctx context.Context, req Request) (drawing.Frame, error)
}

// Request is one generation request.
//
// Seed and Border are dedicated fields rather than Values entries: an int64
// seed does not survive a float64 round-trip cleanly, and a border is a
// polygon, not a scalar.
type Request struct {
	Seed int64

	// Page is the sheet to draw on, in millimetres. Zero means the patch's own
	// default.
	//
	// Separate from Border, and from Values, on purpose. Page shape is a
	// property of the paper rather than of the pattern — the same drawing makes
	// sense on A4 or on a square — so it belongs to the request, like the seed,
	// not to the patch's parameter list. Keeping it out of Values also stops a
	// control surface from mapping a MIDI knob onto the paper size, which is
	// never what anyone wants.
	Page drawing.Size

	// Border confines the drawing to an explicit polygon. When set it overrides
	// Page entirely, including any margin the patch would have applied.
	Border []drawing.Vec2

	Values Values
}

// PageOr returns the requested page size, or the given default when none was
// specified.
//
// Patches should route their default sheet through this rather than reading
// their own constants, so page size works uniformly across every patch.
func (r Request) PageOr(w, h float64) (float64, float64) {
	if r.Page.IsZero() {
		return w, h
	}
	return r.Page.W, r.Page.H
}

// Regioner is implemented by patches that divide space into regions before
// drawing into them. It is deliberately optional and separate from Patch: it
// exposes the regions a patch computed, so one patch's cells can become another
// patch's borders, without every patch having to have regions at all.
//
// A flow-field or growth patch has no meaningful regions and simply does not
// implement it.
type Regioner interface {
	Regions(ctx context.Context, req Request) ([]drawing.Vec2Ring, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Patch{}
)

// Register adds p to the registry. Patches call this from init(). It panics on
// a duplicate or empty name, since both are programming errors that would
// otherwise surface as a confusing missing patch at runtime.
func Register(p Patch) {
	mu.Lock()
	defer mu.Unlock()
	name := p.Name()
	if name == "" {
		panic("patch: Register called with empty name")
	}
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("patch: duplicate registration of %q", name))
	}
	registry[name] = p
}

// Get returns the registered patch with the given name.
func Get(name string) (Patch, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("patch: no patch named %q (have %v)", name, namesLocked())
	}
	return p, nil
}

// Names lists every registered patch, sorted.
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
