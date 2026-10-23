package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) diagramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "diagram",
		Annotations: uses(nil, nil),
		Short:       "Draw diagrams from specs of parts at absolute coordinates",
		Args:        usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(a.diagramRenderCmd(), a.diagramExampleCmd())
	return cmd
}

func (a *app) diagramRenderCmd() *cobra.Command {
	var noAssets bool
	cmd := &cobra.Command{
		Use:         "render <spec.yaml>...",
		Annotations: uses(nil, []setting{iconsSetting}),
		Short:       "Check diagram specs and print them as one HTML fragment of inline SVG",
		Long: `Check each spec and print the diagrams, in order, as one HTML fragment of inline SVG
to paste into a page. A mistake in a spec's shape stops that spec at the first one; layout
mistakes (overflowing text, overlaps, edges through boxes, crossings …) are all reported.
An icon part takes the icons that floe icons fetch <set> keeps.
Every mistake is printed as file:line:column, and nothing is drawn until all are fixed.
The ids of the diagrams drawn together must differ.

The fragment starts with the diagrams' style and the script that zooms them (once per page
is enough; more does no harm). --no-assets leaves them out.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.render(args, !noAssets)
		},
	}
	cmd.Flags().BoolVar(&noAssets, "no-assets", false, "print the diagrams without their style and zoom script")
	return cmd
}

func (a *app) render(files []string, assets bool) error {
	files = a.namedFiles(files)
	// The mistakes of each file, in the order the files were given; those about all of the diagrams (icons
	// not fetched) after them.
	told := make([]usererr.List, len(files)+1)
	specs := make([]*diagram.Spec, 0, len(files))
	for i, file := range files {
		src, err := a.readArg(file)
		var s *diagram.Spec
		if err == nil {
			s, err = diagram.Parse(file, src)
		}
		if err != nil {
			if failed := told[i].Add(err); failed != nil {
				return failed //nolint:wrapcheck // already says what failed
			}
			continue
		}
		specs = append(specs, s)
	}
	svgs, err := diagram.Render(specs, icons.New(a.cfg.icons))
	var drawn usererr.List
	if failed := drawn.Add(err); failed != nil {
		return failed //nolint:wrapcheck // already says what failed
	}
	for _, e := range drawn {
		i := slices.Index(files, e.File)
		if i < 0 {
			i = len(files)
		}
		told[i] = append(told[i], e)
	}
	if err := slices.Concat(told...).Err(); err != nil {
		return err
	}
	out := strings.Join(svgs, "\n")
	if assets {
		head, err := diagram.Assets()
		if err != nil {
			return err //nolint:wrapcheck // already says what failed
		}
		out = head + "\n" + out
	}
	a.out("%s", out)
	return nil
}

func (a *app) diagramExampleCmd() *cobra.Command {
	var lang string
	cmd := &cobra.Command{
		Use:         "example [kind]",
		Annotations: uses(nil, nil),
		Short:       "List the built-in example specs, or print the one of a kind",
		Long: `Without a kind, list the built-in examples: one per kind of diagram, each a spec that
passes the checks. With a kind, print its spec, whose comments explain its grid.

The examples are in English, or in the language of --lang. An example not written in that language
is printed in the language without its region (ja-JP to ja), else in English.`,
		Args: usage(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := i18n.English
			if cmd.Flags().Changed("lang") {
				if err := checkLang(lang); err != nil {
					return err
				}
				if matched, ok := i18n.Match(lang); ok {
					in = matched
				}
			}
			examples, err := diagram.Examples(in)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			if len(args) == 0 {
				for _, e := range examples {
					a.out("%s\t%s", e.Name, e.Summary)
				}
				return nil
			}
			i := slices.IndexFunc(examples, func(e diagram.Example) bool { return e.Name == args[0] })
			if i < 0 {
				names := make([]string, len(examples))
				for i, e := range examples {
					names[i] = e.Name
				}
				return usageError{err: usererr.New("diagram_unknown_example", args[0], strings.Join(names, ", "))}
			}
			a.out("%s", strings.TrimSuffix(string(examples[i].Spec), "\n"))
			return nil
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "", "the language of the examples, as a BCP 47 tag (en, ja)")
	return cmd
}
