package typecloud

import (
	"context"
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/patch"
)

// hashFrame is a fingerprint of every point of a frame.
func hashFrame(f drawing.Frame) string {
	h := fnv.New64a()
	for _, p := range f.Paths {
		fmt.Fprintf(h, "|%v", p.Closed)
		for _, q := range p.Pts {
			fmt.Fprintf(h, " %.4f,%.4f", q.X, q.Y)
		}
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// The line-font mode must not drift: these fingerprints pin what each seed
// draws. They were retaken when the line fonts grew from 4 to 11 (a seed picks
// its fonts from the whole set, so it draws different ones than before);
// change them only on purpose.
func TestStyleZeroUnchanged(t *testing.T) {
	want := map[int64]string{1: "e6ec3993c58e37a1", 2: "5d7e22f26ea42846", 3: "0a0de9856b9ab6cb"}
	for seed, hash := range want {
		f, err := New().Generate(context.Background(), patch.Request{Seed: seed,
			Strings: patch.Strings{"text": "Typecloud;pen plotter;ink;lines of text"}})
		if err != nil {
			t.Fatal(err)
		}
		if got := hashFrame(f); got != hash {
			t.Errorf("seed %d: line-font drawing drifted: %s, want %s", seed, got, hash)
		}
	}
}
