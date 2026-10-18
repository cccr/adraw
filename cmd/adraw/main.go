// Command adraw generates drawings from the command line.
//
// It is the first UI layer over the engine. It carries no per-patch knowledge:
// parameters are discovered from each patch's ParamSpec list, so adding a patch
// never means touching this file.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cccr/adraw/internal/cli"
	"github.com/cccr/adraw/pkg/optimize"
	"github.com/cccr/adraw/pkg/patch"
	"github.com/cccr/adraw/pkg/render"
	"github.com/cccr/adraw/ui/web"

	_ "github.com/cccr/adraw/patches"
	_ "github.com/cccr/adraw/renderers"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "adraw:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command given")
	}
	switch args[0] {
	case "list":
		return cmdList()
	case "params":
		return cmdParams(args[1:])
	case "render":
		return cmdRender(args[1:])
	case "serve":
		return cmdServe(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `adraw - generate controllable drawings

Usage:
  adraw list                      list available patches and renderers
  adraw params <patch>            show a patch's tunable parameters
  adraw render [flags]            generate a drawing
  adraw serve [--addr host:port]  run the web UI and JSON API
              [--config file]     gallery categories and tile previews

Render flags:
  --patch <name>    patch to run (required)
  --seed <int>      random seed (default 1)
  --set key=value   set a parameter; repeatable
  --out <file>      output file; the renderer is inferred from its extension
  --border <file>   JSON [[x,y],...] polygon to draw inside, in mm
  --page <WxH>      page size in mm, e.g. 210x297 (default A4 landscape)

Examples:
  adraw render --patch tangle --seed 42 --set cell=14 --out out.svg
  adraw serve
`)
}

func cmdList() error {
	fmt.Println("Patches:")
	for _, n := range patch.Names() {
		p, err := patch.Get(n)
		if err != nil {
			return err
		}
		fmt.Printf("  %-8s %s\n", n, p.Description())
	}
	fmt.Println("\nRenderers:")
	for _, n := range render.Names() {
		r, err := render.Get(n)
		if err != nil {
			return err
		}
		fmt.Printf("  %-8s .%s\n", n, r.Ext())
	}
	return nil
}

func cmdParams(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: adraw params <patch>")
	}
	p, err := patch.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("%s - %s\n\n", p.Name(), p.Description())
	cli.PrintParams(os.Stdout, p)
	return nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "localhost:8080", "address to listen on")
	config := fs.String("config", "adraw.json", "gallery config file; edits are picked up without a restart")
	if err := fs.Parse(args); err != nil {
		return err
	}
	web.SetConfigPath(*config)
	fmt.Printf("adraw serving on http://%s\n", *addr)
	return http.ListenAndServe(*addr, web.Handler())
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	name := fs.String("patch", "", "patch to run")
	seed := fs.Int64("seed", 1, "random seed")
	out := fs.String("out", "", "output file")
	borderPath := fs.String("border", "", "JSON file of [[x,y],...] to draw inside; default is the page")
	page := fs.String("page", "", "page size in mm as WxH, e.g. 210x297; default is A4 landscape")
	vals := &cli.ValueFlag{}
	fs.Var(vals, "set", "set a parameter as key=value; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--patch is required")
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}

	p, err := patch.Get(*name)
	if err != nil {
		return err
	}
	if err := cli.Validate(vals.Values, p.Params()); err != nil {
		return err
	}
	r, err := render.ByExt(filepath.Ext(*out))
	if err != nil {
		return err
	}

	border, err := cli.LoadBorder(*borderPath)
	if err != nil {
		return err
	}

	size, err := cli.ParsePage(*page)
	if err != nil {
		return err
	}

	frame, err := p.Generate(context.Background(), patch.Request{
		Seed:   *seed,
		Page:   size,
		Border: border,
		Values: vals.Values,
	})
	if err != nil {
		return err
	}

	// Shorten the pen-up travel before writing. Purely a reordering: the
	// strokes are identical, they are just drawn in a saner sequence.
	frame = optimize.Reorder(frame)

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := r.Render(f, frame); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	fmt.Printf("wrote %s: %d paths via %s (travel %s mm, was %s)\n",
		*out, len(frame.Paths), r.Name(),
		frame.Meta["travel_after_mm"], frame.Meta["travel_before_mm"])
	return nil
}
