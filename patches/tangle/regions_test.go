package tangle

import (
	"context"
	"math"
	"testing"

	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
)

func TestRegionsTileTheBorder(t *testing.T) {
	req := patch.Request{Seed: 2, Values: patch.Values{"cell": 25, "margin": 10}}
	regions, err := New().Regions(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) < 10 {
		t.Fatalf("got %d regions", len(regions))
	}
	var sum float64
	for _, r := range regions {
		sum += geom.Area(geom.Polygon(r))
	}
	want := geom.Area(geom.RectPolygon(297, 210, 10))
	if math.Abs(sum-want) > 1e-6 {
		t.Errorf("regions cover %.3f, border is %.3f", sum, want)
	}
}

func TestRegionsMatchWhatGenerateDrew(t *testing.T) {
	// Regions and Generate must agree, or composing on regions would draw into
	// cells the patch never actually used.
	req := patch.Request{Seed: 9, Values: patch.Values{"cell": 30}}
	regions, err := New().Regions(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	f, err := New().Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	// Every ring drawn must lie inside some region.
	outside := 0
	for _, p := range f.Paths {
		in := false
		for _, r := range regions {
			if geom.Contains(geom.Polygon(r), p.Pts[0]) {
				in = true
				break
			}
		}
		if !in {
			outside++
		}
	}
	if outside > 0 {
		t.Errorf("%d of %d paths start outside every region", outside, len(f.Paths))
	}
}
