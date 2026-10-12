package cli

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) diagramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagram",
		Short: "Draw diagrams from specs of parts at absolute coordinates",
		Args:  usage(cobra.NoArgs),
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
		Use:   "render <spec.yaml>...",
		Short: "Check diagram specs and print them as one HTML fragment of inline SVG",
		Long: `Check each spec and print the diagrams, in order, as one HTML fragment of inline SVG
to paste into a page. A mistake in a spec's shape stops that spec at the first one; layout
mistakes (overflowing text, overlaps, edges through boxes, crossings …) are all reported.
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
	var problems diagram.Problems
	specs := make([]*diagram.Spec, 0, len(files))
	for _, file := range files {
		abs := file
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(a.env.Dir, abs)
		}
		src, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's spec is the point
		if err != nil {
			return usererr.CannotRead(file, err)
		}
		s, err := diagram.Parse(file, src)
		var mistake *diagram.Error
		if errors.As(err, &mistake) {
			problems = append(problems, mistake)
			continue
		}
		if err != nil {
			return err //nolint:wrapcheck // already says what failed
		}
		specs = append(specs, s)
	}
	out, err := diagram.Render(specs)
	var more diagram.Problems
	if errors.As(err, &more) {
		problems = append(problems, more...)
	} else if err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	if len(problems) > 0 {
		// The mistakes of each file together, in the order the files were given.
		order := map[string]int{}
		for i, f := range files {
			if _, ok := order[f]; !ok {
				order[f] = i
			}
		}
		slices.SortStableFunc(problems, func(x, y *diagram.Error) int { return cmp.Compare(order[x.File], order[y.File]) })
		return problems
	}
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
		Use:   "example [kind]",
		Short: "List the built-in example specs, or print the one of a kind",
		Long: `Without a kind, list the built-in examples: one per kind of diagram, each a spec that
passes the checks. With a kind, print its spec, whose comments explain its grid.

The examples are written in the language of floe's messages (FLOE_LANG, LC_ALL, LC_MESSAGES,
LANG), or in --lang. An example not written in that language is printed in the language without
its region (ja-JP to ja), else in English.`,
		Args: usage(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := a.cat.Lang()
			if cmd.Flags().Changed("lang") {
				if err := checkLang(lang); err != nil {
					return err
				}
				// A language without messages has no examples either: those are in English.
				in = i18n.English
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
