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
	return &cobra.Command{
		Use:   "render <spec.yaml>...",
		Short: "Check diagram specs and print them as one HTML fragment of inline SVG",
		Long: `Check each spec and print the diagrams, in order, as one HTML fragment of inline SVG
to paste into a page. A mistake in a spec's shape stops that spec at the first one; layout
mistakes (overflowing text, overlaps, edges through boxes, crossings …) are all reported.
Every mistake is printed as file:line:column, and nothing is drawn until all are fixed.
The ids of the diagrams drawn together must differ.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.render(args)
		},
	}
}

func (a *app) render(files []string) error {
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
	a.out("%s", out)
	return nil
}

func (a *app) diagramExampleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "example [kind]",
		Short: "List the built-in example specs, or print the one of a kind",
		Long: `Without a kind, list the built-in examples: one per kind of diagram, each a spec that
passes the checks. With a kind, print its spec, whose comments explain its grid.`,
		Args: usage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			examples, err := diagram.Examples()
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
}
