package cli

import (
	"cmp"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <data.yaml>...",
		Short: "Check data files as their pages are drawn, without writing them",
		Long: `Check each data file as floe draws its page: its YAML, its template and theme, the template's
schema, its checks and computed values, and the page drawn. Every mistake of every file is
printed as file:line:column: message, and the exit code is 1 when there is one. Nothing is
written, and nothing is printed when the files are right.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.check(args)
		},
	}
}

func (a *app) check(paths []string) error {
	k, err := kit.Floe()
	if err != nil {
		return err //nolint:wrapcheck // floe's own, which its tests check
	}
	var found usererr.List
	for _, p := range a.args(paths, "cli_wants_data") {
		_, err := drawFile(k, p)
		if failed := found.Add(err); failed != nil {
			return failed
		}
	}
	return found.Err()
}

// drawFile draws the page of the data file p with k, titled, without a title:, by the slug of its name, else,
// for a name without a letter a slug takes, by the name itself; or returns its mistakes.
func drawFile(k *kit.Kit, p arg) ([]byte, error) {
	// An empty path has no name to check; one that is nowhere has, and is told with it.
	if p.name == "" {
		return nil, p.err
	}
	notData := nameMistake(p.name)
	if notData != nil || p.err != nil {
		return nil, usererr.Collect(notData, p.err) //nolint:wrapcheck // mistakes for the user as they are
	}
	src, err := p.read()
	if err != nil {
		return nil, err
	}
	untitled := cmp.Or(files.SlugFromFile(p.name), baseName(p.name))
	//nolint:wrapcheck // its mistakes, or what failed
	return tmpl.Draw(k, p.name, src, tmpl.Options{Theme: "", Untitled: untitled, ImagesDir: dirs.Dir(p.abs)})
}

// nameMistake refuses file by its name: one that is no data file, or of no name before its extension, which
// names its page; nil for none.
func nameMistake(file string) error {
	switch {
	case !files.IsDataFile(file):
		return usererr.New("cli_check_not_data", file)
	case baseName(file) == "":
		return usererr.New("cli_data_no_name", file)
	}
	return nil
}

// baseName is the name of file without its extension: what floe render names its page by, and the title of
// last resort.
func baseName(file string) string {
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}
