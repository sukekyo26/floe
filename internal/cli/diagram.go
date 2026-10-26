package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

func (a *app) diagramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagram",
		Short: "Draw diagrams from specs of parts on a canvas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
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
overflowing text, overlaps, edges through boxes, crossings … Every mistake of every spec is printed
as file:line:column: message, and nothing is drawn until all are fixed.
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
				head, err := diagram.Assets()
				if err != nil {
					return err //nolint:wrapcheck // floe's own, which its tests check
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
	d, err := diagram.Own()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own schema, which its tests check
	}
	var store diagram.Icons
	if dir, dirErr := a.iconsDir(); dirErr != nil {
		store = noIcons{err: dirErr}
	} else {
		store = icons.New(dir)
	}
	var found usererr.List
	var out []string
	for i, p := range a.args(paths, "cli_wants_spec") {
		got, err := diagramOf(d, p, diagram.Options{Lang: i18n.Lang(lang), Icons: store, Seq: i + 1}, with)
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

func diagramOf(
	d *diagram.Drawer, p arg, o diagram.Options, with func(*diagram.Drawer, any, diagram.Options) (string, error),
) (string, error) {
	src, err := p.read()
	if err != nil {
		return "", err
	}
	// A file of nothing is a spec of nothing, as a data file of nothing is: the schema tells what it lacks.
	var v any = map[string]any{}
	root, err := yamlpos.Parse(p.name, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
	case err != nil:
		return "", err //nolint:wrapcheck // a mistake at its line and column
	default:
		if err = yamlpos.Decode(p.name, root, &v); err != nil {
			return "", err //nolint:wrapcheck // as above
		}
	}
	got, err := with(d, doc.Ints(v), o)
	return got, yamlpos.Place(err, p.name, root, nil) //nolint:wrapcheck // mistakes placed
}

// noIcons are no icons to look up: there is no directory for them, which a diagram with an icon is told.
type noIcons struct{ err error }

func (n noIcons) Lookup(string) ([]byte, error) {
	return nil, n.err
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
			examples, err := diagram.Examples()
			if err != nil {
				return err //nolint:wrapcheck // floe's own, which its tests check
			}
			if len(args) == 0 {
				for _, e := range examples {
					a.print("%s\t%s", e.Name, e.Summary)
				}
				return nil
			}
			i := slices.IndexFunc(examples, func(e diagram.Example) bool { return e.Name == args[0] })
			if i < 0 {
				names := make([]string, len(examples))
				for i, e := range examples {
					names[i] = e.Name
				}
				return usererr.New("diagram_unknown_example", args[0], strings.Join(names, ", "))
			}
			a.print("%s", strings.TrimSuffix(string(examples[i].Spec), "\n"))
			return nil
		},
	}
}
