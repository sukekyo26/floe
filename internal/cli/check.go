package cli

import (
	"cmp"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
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
	found := make(mistakes, 0, len(files))
	for _, file := range files {
		found = append(found, checkFile(a, file)...)
	}
	if len(found) > 0 {
		return found
	}
	return nil
}

// checkFile returns the mistakes of file: each at its line and column, or one about the whole file.
func checkFile(a *app, file string) []error {
	if !store.IsDataFile(file) {
		return []error{usererr.New("cli_check_not_data", file)}
	}
	src, err := a.readArg(file)
	if err != nil {
		return []error{err}
	}
	d, err := a.templates().Read(file, src, "", a.env.Dir)
	var all yamlpos.Problems
	switch {
	case errors.As(err, &all):
		found := make([]error, len(all))
		for i, e := range all {
			found[i] = e
		}
		return found
	case err != nil:
		return []error{err}
	}
	// Drawing is the last stage. Data that passed its schema fails here only by the template's mistake.
	if _, err := d.Render(cmp.Or(d.Title, file)); err != nil {
		return []error{err}
	}
	return nil
}

// mistakes are the mistakes of every file checked, one per line, in the order of the files.
type mistakes []error

func (m mistakes) Error() string {
	return m.Localize(i18n.EnglishCatalog())
}

func (m mistakes) Localize(c *i18n.Catalog) string {
	lines := make([]string, len(m))
	for i, e := range m {
		lines[i] = e.Error()
		var l i18n.Localizer
		if errors.As(e, &l) {
			lines[i] = l.Localize(c)
		}
	}
	return strings.Join(lines, "\n")
}
