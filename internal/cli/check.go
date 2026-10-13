package cli

import (
	"cmp"
	"errors"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

func (a *app) checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <data.yaml>...",
		Short: "Check data files as floe publish would, without publishing them",
		Long: `Check each data file as floe publish does before it draws the page: its YAML, its template
and theme, the template's schema, and drawing the page. Every mistake of every file is printed
as file:line:column: message, and the exit code is 1 when there is one. Nothing is published,
the server is not started, and nothing is printed when the files are right.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.check(args)
		},
	}
}

func (a *app) check(files []string) error {
	var problems yamlpos.Problems
	for _, file := range files {
		if !store.IsDataFile(file) {
			return usererr.New("cli_check_not_data", file)
		}
		src, err := a.readArg(file)
		if err != nil {
			return err
		}
		d, err := tmpl.Read(file, src)
		var all yamlpos.Problems
		var one *yamlpos.Error
		switch {
		case errors.As(err, &all):
			problems = append(problems, all...)
			continue
		case errors.As(err, &one):
			problems = append(problems, one)
			continue
		case err != nil:
			return err //nolint:wrapcheck // already says what failed
		}
		// Drawing is the last stage. Data that passed its schema fails here only by the template's mistake.
		if _, err := d.Render(cmp.Or(d.Title, file), ""); err != nil {
			return err //nolint:wrapcheck // a usererr naming the file and the template
		}
	}
	if problems != nil {
		return problems
	}
	return nil
}
