package typecloud

import (
	"context"
	"testing"
	"time"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/patch"
)

// In the browser the engine runs on the main thread, so a slow draw freezes the
// tab. A 2 m square sheet (the page form allows it) is the worst case.
func TestLargestSheetDrawsQuickly(t *testing.T) {
	start := time.Now()
	_, err := New().Generate(context.Background(), patch.Request{Seed: 1,
		Page:   drawing.Size{W: 2000, H: 2000},
		Values: patch.Values{"gap": 0.5, "fill": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 800*time.Millisecond {
		t.Errorf("2000x2000 mm took %v, want under 800ms natively", d)
	}
}

func TestSolidLettersAreBoundedToo(t *testing.T) {
	many := ""
	for i := 0; i < 1500; i++ {
		many += "word;"
	}
	for name, req := range map[string]patch.Request{
		"2 m sheet":   {Seed: 1, Page: drawing.Size{W: 2000, H: 2000}, Values: patch.Values{"style": 1, "gap": 0.5, "fill": 1}},
		"1500 chunks": {Seed: 1, Values: patch.Values{"style": 1}, Strings: patch.Strings{"text": many}},
		"tiny page":   {Seed: 1, Page: drawing.Size{W: 10, H: 10}, Values: patch.Values{"style": 1}},
	} {
		start := time.Now()
		f, err := New().Generate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%s took %v", name, d)
		}
		if len(f.Paths) == 0 {
			t.Errorf("%s: no paths", name)
		}
	}
}
