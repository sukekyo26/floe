package cli

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/goccy/go-yaml/ast"
	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/draw"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

func (a *app) diagramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagram",
		Short: "Draw diagrams from specs of parts on a canvas",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.diagramRenderCmd(), a.diagramInspectCmd(), a.diagramExampleCmd())
	return cmd
}

func (a *app) diagramRenderCmd() *cobra.Command {
	var noAssets bool
	var lang string
	cmd := &cobra.Command{
		Use:   "render <spec.yaml>...",
		Short: "Check diagram specs and print them as one HTML fragment of inline SVG",
		Long: `Check each spec and print the diagrams, in order, as one HTML fragment of inline SVG to paste
into a page, their text in the language of --lang (en when left out). A spec is checked against
the diagram's schema, then laid out once for every language its text is written in and checked:
overflowing text, overlaps, edges through boxes, crossings … (frames may cross one another, and a
frame's label is held to the frame's width, not its height or a canvas written). Every mistake of every
spec is printed as file:line:column: message (file: message for one of the whole spec, such as a
key it lacks), and nothing is drawn until all are fixed. A width or height left out is as far as
what is drawn reaches, and 20 more; raw SVG is not measured, so write both with a raw part.
An icon part takes the icons that floe icons fetch <set> keeps.

The fragment starts with the diagrams' style and the script that zooms them (once per page is
enough; more does no harm). --no-assets leaves them out.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			svgs, err := a.diagrams(args, lang, "cli_wants_spec", diagramOf)
			if err != nil {
				return err
			}
			out := strings.Join(svgs, "\n")
			if !noAssets {
				head, err := diagramAssets()
				if err != nil {
					return err
				}
				out = head + "\n" + out
			}
			a.print("%s", out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noAssets, "no-assets", false, "print the diagrams without their style and zoom script")
	cmd.Flags().StringVar(&lang, "lang", "en", "the language to write the diagrams' text in, as a tag (en, ja)")
	return cmd
}

func (a *app) diagramInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <spec.yaml|data.yaml>...",
		Short: "Check diagrams, of specs or of data files, and print where each part went, without drawing them",
		Long: `Check each diagram as floe diagram render does, and print where each part went. A file with a
key template at its top is a data file: its diagrams are read after the template's schema, in the
order they are written. The checks of its template and parts do not run, and the rest of its page
is not checked (its items of format html, its images, what its parts' functions refuse): floe check
checks the whole page. A diagram a part of one's own draws with {{diagram}} is not among them. Any
other file is a spec.

Each diagram prints a line file:line:column: diagram "title" WIDTHxHEIGHT (file: for a spec), then,
a line each: a part's area, a frame's heading, an edge's path and the ends it joins, a label's area
and the circles it is in. It lets one check that edges meet the rows they mean and labels sit in
their areas, without seeing the drawing.

Where is printed on stdout and mistakes on stderr, and the exit code is 1 when there is one. A
diagram refused by the checks is printed too, as a guide: fixing it may move its parts and edges. A
diagram that does not get as far as its layout (a mistake of its YAML or schema, an example there
is none of) prints no line.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			own := sync.OnceValues(func() (*kit.Kit, error) { return a.kit(true) })
			// The layout is one for every language: inspected in English.
			found, err := a.diagrams(args, "en", "cli_wants_spec_or_data",
				func(d *diagram.Drawer, p arg, o diagram.Options) (string, error) { return inspectFile(d, own, p, o) })
			if len(found) > 0 {
				a.print("%s", strings.TrimSuffix(strings.Join(found, ""), "\n"))
			}
			return err
		},
	}
}

// diagrams does one with each file, in the language lang, each numbered by its place among them, wants the key of what
// to give: what each gave, and every mistake of every file, at its line and column.
func (a *app) diagrams(
	paths []string, lang, wants string, one func(*diagram.Drawer, arg, diagram.Options) (string, error),
) ([]string, error) {
	d, err := draw.Diagrams()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own schema, which its tests check
	}
	store := a.iconStore()
	var found usererr.List
	var out []string
	for i, p := range a.args(paths, wants) {
		got, err := one(d, p, diagram.Options{Lang: i18n.Tag(lang), Icons: store, Seq: i + 1})
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
		if got != "" {
			out = append(out, got)
		}
	}
	return out, found.Err()
}

// diagramAssets are what a page holds once for the diagrams of floe diagram render: the script that zooms them,
// default's palette and the tones' colours on the diagrams' roots alone, in its layer, and the CSS of floe's part
// diagram, unscoped, as a page without floe's parts takes it.
// LIMIT: a browser that does not read @scope drops the palette, and the diagrams are drawn without colours; the
// docs ask for the browsers the README names.
func diagramAssets() (string, error) {
	script, err := diagram.Script()
	if err != nil {
		return "", err //nolint:wrapcheck // floe's own, which its tests check
	}
	k, err := kit.Floe()
	if err != nil {
		return "", err //nolint:wrapcheck // as above
	}
	p, ok := k.Part("diagram")
	if !ok {
		return "", errors.New("diagram: floe has no part diagram") //nolint:err113 // floe's own mistake
	}
	t, ok := k.Theme(kit.DefaultTheme)
	if !ok {
		return "", errors.New("diagram: floe has no theme " + kit.DefaultTheme) //nolint:err113 // floe's own mistake
	}
	// The page around the diagrams keeps its own palette. In a layer, so that the page's rules for the diagrams win:
	// out of one, the palette's :scope, as near as can be, would win over those as specific.
	tones, err := k.Tones()
	if err != nil {
		return "", err //nolint:wrapcheck // as above
	}
	colours := slices.Concat(t.Palette(), draw.Tones(tones))
	palette := draw.Sheet("floe.palette", draw.InScope("("+diagram.Roots+")", colours))
	return script + "\n<style>" + palette + "</style>\n<style>" + draw.Sheet("", p.CSS()) + "</style>", nil
}

// diagramOf draws the spec file p.
func diagramOf(d *diagram.Drawer, p arg, o diagram.Options) (string, error) {
	_, root, v, err := specOf(p)
	if err != nil {
		return "", err
	}
	svg, err := d.Draw(doc.Ints(v), o)
	return string(svg), yamlpos.Place(err, p.name, root, nil) //nolint:wrapcheck // mistakes placed
}

// specOf reads the file p as doc.Read reads a data file: a file of nothing is a spec of nothing, whose lacks the schema
// tells.
func specOf(p arg) ([]byte, *ast.MappingNode, map[string]any, error) {
	src, err := p.read()
	if err != nil {
		return nil, nil, nil, err
	}
	root, v, err := yamlpos.ParseMap(p.name, src)
	if err != nil {
		return nil, nil, nil, err //nolint:wrapcheck // a mistake at its line and column
	}
	return src, root, v, nil
}

// inspectFile inspects the diagrams of the file p, a data file by the key template at its top, read with own, or else
// a spec, each under the place it is written at; and every mistake.
// LIMIT: the key alone tells them: a data file lacking it is told as a spec is, in words that do not say it lacks it,
// and a spec with it is told as a page. Telling them by more keys is a special case for a mistake plain to see.
func inspectFile(d *diagram.Drawer, own func() (*kit.Kit, error), p arg, o diagram.Options) (string, error) {
	src, root, v, err := specOf(p)
	if err != nil {
		return "", err
	}
	if _, isData := v["template"]; !isData {
		got, mistakes := d.Inspect(doc.Ints(v), o)
		pos, _ := yamlpos.Locate(root, nil)
		//nolint:wrapcheck // mistakes placed
		return placedAt(p.name, pos, got), yamlpos.Place(mistakes, p.name, root, nil)
	}
	k, err := own()
	if err != nil {
		return "", err
	}
	data, items, err := tmpl.Diagrams(k, p.name, src)
	if err != nil {
		return "", err //nolint:wrapcheck // mistakes of the data file, or floe's failure
	}
	var out strings.Builder
	var found usererr.List
	for _, it := range items {
		got, told := d.Inspect(it.Value, o)
		out.WriteString(placedAt(p.name, data.PosOf(it.At), got))
		var mistakes usererr.List
		if failed := mistakes.Add(told); failed != nil {
			return "", failed
		}
		if failed := found.Add(data.PlaceItem(it.At, mistakes)); failed != nil {
			return "", failed
		}
	}
	return out.String(), found.Sorted().Err()
}

// placedAt is what Inspect told, got, its first line, the diagram's, after where in file it is written, as a mistake's
// place is written: "" for nothing told.
func placedAt(file string, pos yamlpos.Pos, got string) string {
	if got == "" {
		return ""
	}
	return usererr.Where(file, pos.Line, pos.Column) + ": " + got
}

func (a *app) diagramExampleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "example [kind]",
		Short: "List the built-in example specs, or print the one of a kind",
		Long: `Without a kind, list the built-in examples: one per kind of diagram, each a spec that passes
the checks. With a kind, print its spec, whose comments explain its grid. Its text is written in
each language it has ({en: …, ja: …}), as a spec may write it.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				examples, err := diagram.Examples()
				if err != nil {
					return err //nolint:wrapcheck // floe's own, which its tests check
				}
				for _, e := range examples {
					a.print("%s\t%s", e.Name, e.Summary)
				}
				return nil
			}
			e, err := diagram.ExampleOf(args[0])
			if err != nil {
				return err //nolint:wrapcheck // the mistake of a kind there is none of, or floe's own
			}
			a.print("%s", strings.TrimSuffix(string(e.Spec), "\n"))
			return nil
		},
	}
}
