package cli

import (
	"cmp"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "check <data.yaml>...",
		Annotations: uses(drawUses),
		Short:       "Check data files as floe publish would, without publishing them",
		Long: `Check each data file as floe publish does before it draws the page: its YAML, its template
and themes, the template's schema, and drawing the page. Every mistake of every file is printed
as file:line:column: message, and the exit code is 1 when there is one. Nothing is published,
the server is not started, and nothing is printed when the files are right.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.check(args)
		},
	}
}

func (a *app) check(files []string) error {
	var found usererr.List
	for _, p := range a.args(files, "cli_wants_data") {
		_, err := drawFile(a, p)
		tell(&found, err)
	}
	return found.Err()
}

// tell adds err to l as usererr.List.Add does, and any other error too, as a mistake of its own: for the
// list of the outcomes of things apart a command tells at its exit, such as the files floe check checks,
// where the failure of one does not hide the others.
func tell(l *usererr.List, err error) {
	if failed := l.Add(err); failed != nil {
		*l = append(*l, usererr.New("cli_failure", failed))
	}
}

// drawFile draws the page of the data file file, titled as floe publish titles it (its title:, else the
// slug of its name, else, for a name without a letter a slug takes, the name itself), or returns its
// mistakes: each at its line and column, or one about the whole file.
func drawFile(a *app, p arg) ([]byte, error) {
	file := p.name
	// An empty path has no name to check; one that is nowhere has, and is told with it.
	if file == "" {
		return nil, p.err
	}
	var notData error
	if !store.IsDataFile(file) {
		notData = usererr.New("cli_check_not_data", file)
	}
	if notData != nil || p.err != nil {
		return nil, usererr.Collect(notData, p.err) //nolint:wrapcheck // usererrs for the user as they are
	}
	src, err := p.read()
	if err != nil {
		return nil, err
	}
	d, err := a.templates().Read(file, src, "", a.env.Dir)
	if err != nil {
		return nil, err //nolint:wrapcheck // its mistakes, or what failed
	}
	// Drawing is the last stage. Data that passed its schema fails here by the template's mistake, or by an
	// item of HTML that changes the page around it, which is told at its line and column.
	page, err := d.Render(cmp.Or(d.Title, store.SlugFromFile(file, a.env.Dir), baseName(file)))
	if err != nil {
		return nil, err //nolint:wrapcheck // its mistakes, or what failed
	}
	return page, nil
}

// baseName is the name of file without its extension: what floe render names its page by, and the title
// of last resort.
func baseName(file string) string {
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}
