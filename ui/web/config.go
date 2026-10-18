package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/cccr/adraw/pkg/patch"
)

// Config groups patches into categories and tunes how each one is previewed on
// the landing page.
//
// It lives in a file rather than in Go so the gallery can be tuned by eye —
// edit, refresh, look again — without a rebuild. Everything in it is optional:
// a patch the file does not mention still appears, in the fallback category,
// previewed with its own defaults. That keeps the rule that registering a patch
// is enough to make it show up everywhere.
type Config struct {
	Categories      []Category             `json:"categories"`
	DefaultCategory string                 `json:"defaultCategory"`
	Patches         map[string]PatchConfig `json:"patches"`
}

// Category is one group on the landing page.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PatchConfig places one patch and tunes its tile.
type PatchConfig struct {
	Category string `json:"category"`

	// Preview overrides the parameters used for the tile only. The editor still
	// opens the patch at its real defaults. Tiles are small: a patch drawn at
	// working density usually reads as a grey rectangle, so the tile wants
	// coarser settings than the drawing does.
	Preview PreviewConfig `json:"preview"`
}

// PreviewConfig is the seed and parameter overrides for a tile.
type PreviewConfig struct {
	Seed   *int64             `json:"seed"`
	Values map[string]float64 `json:"values"`
}

// fallbackCategory is used when the config names none.
const fallbackCategory = "Other"

var (
	cfgMu   sync.Mutex
	cfgPath string
	cfgData []byte
)

// SetConfigPath tells the server where to read its gallery config from. An
// empty path, or a missing file, means defaults throughout.
func SetConfigPath(path string) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgPath = path
	cfgData = nil
}

// SetConfigData supplies the gallery config as bytes rather than a path, for
// hosts with no filesystem to read from — the WebAssembly build, where the
// browser fetches adraw.json and hands it over. It takes precedence over any
// path, and nil means defaults throughout.
func SetConfigData(raw []byte) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgData = raw
	cfgPath = ""
}

// loadConfig re-reads the file on every call.
//
// Deliberately not cached: the whole point of putting this in a file is tuning
// the gallery by eye, and a cache would mean restarting the server after every
// tweak. The file is a few hundred bytes and the gallery is not a hot path.
func loadConfig() (*Config, error) {
	cfgMu.Lock()
	path, raw := cfgPath, cfgData
	cfgMu.Unlock()

	cfg := &Config{Patches: map[string]PatchConfig{}}
	source := "config data"
	if raw == nil {
		if path == "" {
			return cfg, nil
		}
		var err error
		raw, err = os.ReadFile(path)
		if os.IsNotExist(err) {
			return cfg, nil // absent config is not an error; defaults apply
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		source = path
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", source, err)
	}
	if cfg.Patches == nil {
		cfg.Patches = map[string]PatchConfig{}
	}
	return cfg, nil
}

// GalleryCategory is one landing-page section with its tiles.
type GalleryCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Patches     []Tile `json:"patches"`
}

// Tile is one patch as the landing page needs it: enough to label it and to
// request its thumbnail.
type Tile struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	PreviewSeed int64              `json:"previewSeed"`
	Preview     map[string]float64 `json:"preview"`
}

// gallery assembles the landing page: every registered patch, grouped.
func gallery(cfg *Config) []GalleryCategory {
	// Categories in the order the config lists them, so the file controls how
	// the page reads top to bottom.
	order := make([]*GalleryCategory, 0, len(cfg.Categories)+1)
	byID := map[string]*GalleryCategory{}
	for _, c := range cfg.Categories {
		g := &GalleryCategory{ID: c.ID, Name: c.Name, Description: c.Description}
		order = append(order, g)
		byID[c.ID] = g
	}

	fallbackID := cfg.DefaultCategory
	if fallbackID == "" {
		fallbackID = "other"
	}

	for _, name := range patch.Names() {
		p, err := patch.Get(name)
		if err != nil {
			continue
		}
		pc := cfg.Patches[name]

		id := pc.Category
		if id == "" {
			id = fallbackID
		}
		g, ok := byID[id]
		if !ok {
			// A patch pointing at an unlisted category, or one the config never
			// mentions, still has to appear somewhere.
			g = &GalleryCategory{ID: id, Name: fallbackCategory}
			if id != fallbackID {
				g.Name = id
			}
			byID[id] = g
			order = append(order, g)
		}

		tile := Tile{
			Name:        p.Name(),
			Description: p.Description(),
			PreviewSeed: 1,
			Preview:     pc.Preview.Values,
		}
		if pc.Preview.Seed != nil {
			tile.PreviewSeed = *pc.Preview.Seed
		}
		g.Patches = append(g.Patches, tile)
	}

	// Drop categories the config declared but nothing landed in.
	out := make([]GalleryCategory, 0, len(order))
	for _, g := range order {
		if len(g.Patches) > 0 {
			out = append(out, *g)
		}
	}
	return out
}

func handleGallery(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, gallery(cfg))
}
