package patch

import (
	"context"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
)

type stub struct{ name string }

func (s stub) Name() string        { return s.name }
func (s stub) Description() string { return "a stub patch" }
func (s stub) Params() []ParamSpec { return nil }
func (s stub) Generate(context.Context, Request) (drawing.Frame, error) {
	return drawing.Frame{}, nil
}

func TestRegistry(t *testing.T) {
	Register(stub{"stub-a"})

	if _, err := Get("stub-a"); err != nil {
		t.Fatalf("Get after Register: %v", err)
	}
	if _, err := Get("absent"); err == nil {
		t.Error("Get of an unregistered name should error")
	}

	var found bool
	for _, n := range Names() {
		if n == "stub-a" {
			found = true
		}
	}
	if !found {
		t.Error("Names() omitted the registered patch")
	}
}

func TestRegisterRejectsDuplicateAndEmpty(t *testing.T) {
	Register(stub{"stub-dup"})
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
			Register(stub{c.name})
		})
	}
}
