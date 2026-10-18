package render

import (
	"io"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
)

type stub struct{ name, ext string }

func (s stub) Name() string                          { return s.name }
func (s stub) Ext() string                           { return s.ext }
func (s stub) Render(io.Writer, drawing.Frame) error { return nil }

func TestRegistryAndByExt(t *testing.T) {
	Register(stub{"stub-svg", "svgx"})

	if _, err := Get("stub-svg"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := Get("absent"); err == nil {
		t.Error("Get of an unregistered name should error")
	}

	// ByExt must accept the extension with or without a leading dot, since it
	// is fed straight from filepath.Ext.
	for _, ext := range []string{"svgx", ".svgx"} {
		if r, err := ByExt(ext); err != nil || r.Name() != "stub-svg" {
			t.Errorf("ByExt(%q) = %v, %v", ext, r, err)
		}
	}
	if _, err := ByExt(".unknown"); err == nil {
		t.Error("ByExt of an unknown extension should error")
	}
}

func TestRegisterRejectsDuplicateAndEmpty(t *testing.T) {
	Register(stub{"stub-dup", "dup"})
	for _, c := range []struct{ name, desc string }{
		{"stub-dup", "duplicate"},
		{"", "empty name"},
	} {
		t.Run(c.desc, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%s) should panic", c.desc)
				}
			}()
			Register(stub{c.name, "x"})
		})
	}
}
