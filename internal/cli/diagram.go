package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/draw"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
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
			svgs, err := a.diagrams(args, lang, func(d *diagram.Drawer, v any, o diagram.Options) (string, error) {
				svg, err := d.Draw(v, o)
				return string(svg), err
			})
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
		Use:   "inspect <spec.yaml>...",
		Short: "Check diagram specs and print where each part went, without drawing them",
		Long: `Check each spec as floe diagram render does, and print, a line each, where each part went:
a part's area, an edge's path and the ends it joins, a label's area and the circles it is in. It
lets one check that edges meet the rows they mean and labels sit in their areas, without seeing
the drawing.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// The layout is one for every language: inspected in English.
			found, err := a.diagrams(args, "en", func(d *diagram.Drawer, v any, o diagram.Options) (string, error) {
				return d.Inspect(v, o)
			})
			if err != nil {
				return err
			}
			a.print("%s", strings.TrimSuffix(strings.Join(found, ""), "\n"))
			return nil
		},
	}
}

// diagrams reads each spec file and does with it, in the language lang, each numbered by its place among them;
// every mistake of every file, at its line and column.
func (a *app) diagrams(paths []string, lang string, with func(*diagram.Drawer, any, diagram.Options) (string, error),
) ([]string, error) {
	d, err := draw.Diagrams()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own schema, which its tests check
	}
	store := a.iconStore()
	var found usererr.List
	var out []string
	for i, p := range a.args(paths, "cli_wants_spec") {
		got, err := diagramOf(d, p, diagram.Options{Lang: i18n.Tag(lang), Icons: store, Seq: i + 1}, with)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
		out = append(out, got)
	}
	if err := found.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// diagramAssets are what a page holds once for the diagrams of floe diagram render: the script that zooms them
// and the CSS of floe's part diagram, unscoped, as a page without floe's parts takes it.
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
	return script + "\n<style>\n" + string(p.CSS()) + "</style>", nil
}

func diagramOf(
	d *diagram.Drawer, p arg, o diagram.Options, with func(*diagram.Drawer, any, diagram.Options) (string, error),
) (string, error) {
	src, err := p.read()
	if err != nil {
		return "", err
	}
	// Read as doc.Read reads a data file: a file of nothing is a spec of nothing, whose lacks the schema tells.
	root, v, err := yamlpos.ParseMap(p.name, src)
	if err != nil {
		return "", err //nolint:wrapcheck // a mistake at its line and column
	}
	got, err := with(d, doc.Ints(v), o)
	return got, yamlpos.Place(err, p.name, root, nil) //nolint:wrapcheck // mistakes placed
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
