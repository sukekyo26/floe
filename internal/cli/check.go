package cli

import (
	"cmp"
	"errors"
	"path/filepath"
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
		_, errs := drawFile(a, file)
		found = append(found, errs...)
	}
	if len(found) > 0 {
		return found
	}
	return nil
}

// drawFile draws the page of the data file file, titled as floe publish titles it (its title:, else the
// slug of its name, else, for a name without a letter a slug takes, the name itself), or returns its
// mistakes: each at its line and column, or one about the whole file.
func drawFile(a *app, file string) ([]byte, []error) {
	if !store.IsDataFile(file) {
		return nil, []error{usererr.New("cli_check_not_data", file)}
	}
	src, err := a.readArg(file)
	if err != nil {
		return nil, []error{err}
	}
	d, err := a.templates().Read(file, src, "", a.env.Dir)
	var all yamlpos.Problems
	switch {
	case errors.As(err, &all):
		found := make([]error, len(all))
		for i, e := range all {
			found[i] = e
		}
		return nil, found
	case err != nil:
		return nil, []error{err}
	}
	// Drawing is the last stage. Data that passed its schema fails here only by the template's mistake.
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	page, err := d.Render(cmp.Or(d.Title, store.SlugFromFile(file, a.env.Dir), name))
	if err != nil {
		return nil, []error{err}
	}
	return page, nil
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
