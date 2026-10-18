package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "adraw.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fetchGallery(t *testing.T) []GalleryCategory {
	t.Helper()
	rec := get(t, "/api/gallery")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out []GalleryCategory
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// tileCount totals the tiles across every category.
func tileCount(cats []GalleryCategory) int {
	var n int
	for _, c := range cats {
		n += len(c.Patches)
	}
	return n
}

func TestGalleryWithoutConfigStillListsEveryPatch(t *testing.T) {
	// The config is a convenience, not a requirement: the rule that registering
	// a patch is enough to make it appear must survive a missing file.
	SetConfigPath("")
	defer SetConfigPath("")

	cats := fetchGallery(t)
	if len(cats) == 0 {
		t.Fatal("no categories returned")
	}
	if got, want := tileCount(cats), len(patchNames(t)); got != want {
		t.Errorf("gallery shows %d tiles, %d patches are registered", got, want)
	}
}

func TestGalleryMissingFileIsNotAnError(t *testing.T) {
	SetConfigPath(filepath.Join(t.TempDir(), "absent.json"))
	defer SetConfigPath("")

	if cats := fetchGallery(t); tileCount(cats) == 0 {
		t.Error("a missing config file emptied the gallery")
	}
}

func TestGalleryGroupsAndOrdersByConfig(t *testing.T) {
	SetConfigPath(writeConfig(t, `{
      "defaultCategory": "other",
      "categories": [
        {"id": "fields", "name": "Field patterns", "description": "noise"},
        {"id": "cells",  "name": "Cell patterns",  "description": "subdivision"},
        {"id": "other",  "name": "Other",          "description": "rest"}
      ],
      "patches": {
        "topo":   {"category": "fields", "preview": {"seed": 9, "values": {"levels": 12}}},
        "flow":   {"category": "fields"},
        "tangle": {"category": "cells"}
      }
    }`))
	defer SetConfigPath("")

	cats := fetchGallery(t)

	// Order follows the file, not the alphabet.
	if cats[0].ID != "fields" {
		t.Errorf("first category is %q, want fields as the file lists it", cats[0].ID)
	}
	if cats[0].Description != "noise" {
		t.Errorf("category description = %q", cats[0].Description)
	}
	if len(cats[0].Patches) != 2 {
		t.Errorf("fields has %d tiles, want 2", len(cats[0].Patches))
	}

	// Unlisted patches must still show up somewhere.
	if got, want := tileCount(cats), len(patchNames(t)); got != want {
		t.Errorf("gallery shows %d tiles, %d registered", got, want)
	}
	var other *GalleryCategory
	for i := range cats {
		if cats[i].ID == "other" {
			other = &cats[i]
		}
	}
	if other == nil || len(other.Patches) == 0 {
		t.Error("patches absent from the config did not land in the default category")
	}

	// Preview overrides and seed come through.
	for _, tile := range cats[0].Patches {
		if tile.Name != "topo" {
			continue
		}
		if tile.PreviewSeed != 9 {
			t.Errorf("preview seed = %d, want 9", tile.PreviewSeed)
		}
		if tile.Preview["levels"] != 12 {
			t.Errorf("preview values = %v", tile.Preview)
		}
	}
}

func TestGalleryDefaultsPreviewSeedWhenUnset(t *testing.T) {
	SetConfigPath(writeConfig(t, `{
      "categories": [{"id": "a", "name": "A"}],
      "patches": {"tangle": {"category": "a"}}
    }`))
	defer SetConfigPath("")

	for _, c := range fetchGallery(t) {
		for _, tile := range c.Patches {
			if tile.PreviewSeed == 0 {
				t.Errorf("%s got preview seed 0; a seed must always be usable", tile.Name)
			}
		}
	}
}

func TestGalleryEmptyCategoriesAreDropped(t *testing.T) {
	SetConfigPath(writeConfig(t, `{
      "defaultCategory": "used",
      "categories": [
        {"id": "used",  "name": "Used"},
        {"id": "empty", "name": "Nothing lands here"}
      ]
    }`))
	defer SetConfigPath("")

	for _, c := range fetchGallery(t) {
		if c.ID == "empty" {
			t.Error("a category with no patches was still rendered")
		}
	}
}

func TestGalleryMalformedConfigIsReported(t *testing.T) {
	SetConfigPath(writeConfig(t, `{ this is not json`))
	defer SetConfigPath("")

	rec := get(t, "/api/gallery")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d, want 500 so a typo in the file is visible", rec.Code)
	}
}

func TestConfigIsRereadPerRequest(t *testing.T) {
	// The reason the file is not cached: it exists to be tuned by eye, and a
	// cache would mean restarting the server after every tweak.
	path := writeConfig(t, `{"categories":[{"id":"a","name":"First"}],
	                         "defaultCategory":"a"}`)
	SetConfigPath(path)
	defer SetConfigPath("")

	if got := fetchGallery(t)[0].Name; got != "First" {
		t.Fatalf("category name = %q", got)
	}
	if err := os.WriteFile(path, []byte(`{"categories":[{"id":"a","name":"Second"}],
	                                      "defaultCategory":"a"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := fetchGallery(t)[0].Name; got != "Second" {
		t.Errorf("category name = %q after editing the file; it was cached", got)
	}
}

func TestConfigDataNeedsNoFile(t *testing.T) {
	// The WebAssembly build has no filesystem: the browser fetches adraw.json
	// and passes its bytes in. They must drive the gallery exactly as the file
	// would.
	SetConfigData([]byte(`{"categories":[{"id":"a","name":"From bytes"}],
	                       "defaultCategory":"a"}`))
	defer SetConfigPath("")

	if got := fetchGallery(t)[0].Name; got != "From bytes" {
		t.Errorf("category name = %q", got)
	}
}

func TestConfigDataMalformedIsReported(t *testing.T) {
	SetConfigData([]byte(`{ this is not json`))
	defer SetConfigPath("")

	rec := get(t, "/api/gallery")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d, want 500", rec.Code)
	}
}
