package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) partCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "part",
		Short: "List the parts, show one, make one of your own, and check yours",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the parts, looks and shared schemas (name, where, description; tab-separated)",
		Long: `List the parts, looks and shared schemas, floe's and your own, one per line: its name, where it
is (floe, or its directory of your own) and its description, tab-separated.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			a.list(k.Parts())
			return nil
		},
	}, &cobra.Command{
		Use:   "show <name> [file]",
		Short: "Show a part, look or shared schema and every file it reads, or one of them",
		Long: `Show a part, a look or a shared schema: its description, where it is, the looks it takes, the
files it reads (files:), and each of them as written under ==> file <==: schema.json, checks.yaml,
part.yaml, part.html, part.css and words.yaml of a part, part.yaml and part.css of a look, <name>.json
of a shared schema. With a file named after, print that file alone, as written but the byte order
mark at its start. When the output is long, read the files: line first, then the file you need:
floe part show code part.html.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			return a.show(k.Parts(), args, "cli_unknown_part", nil)
		},
	}, &cobra.Command{
		Use:   "new <name>",
		Short: "Make a part of your own: the smallest that floe part check takes, to fill in",
		Long: `Make the part <name> in FLOE_PARTS_DIR (else floe/parts under XDG_CONFIG_HOME, or ~/.config): a
schema.json, a part.html of its box and an empty part.css, which floe part check takes. Nothing is
written over: with anything of that name there, nothing is made.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.newOwn(kit.KindPart, args[0])
		},
	}, &cobra.Command{
		Use:   "check",
		Short: "Check the parts, looks, shared schemas and themes of your own",
		Long: `Read the parts, looks and shared schemas under FLOE_PARTS_DIR and the themes under
FLOE_THEMES_DIR (else floe/parts and floe/themes under XDG_CONFIG_HOME, or ~/.config), over
floe's own, and print every mistake of them. The exit code is 1 when there is one; nothing is
printed when they are right. A mistake of one of your own stops floe check and floe render too.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := a.kit(false)
			return err
		},
	})
	return cmd
}

func (a *app) templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "List the templates, show one, make one of your own, and check yours",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the templates (name, where, description; tab-separated)",
		Long: `List the templates, floe's and your own, one per line: its name, where it is (floe, or its
directory of your own) and its description, tab-separated. One of your own hides floe's of its name.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := a.kit(true)
			if err != nil {
				return err
			}
			a.list(k.Templates())
			return nil
		},
	}, &cobra.Command{
		Use:   "show <name> [file]",
		Short: "Show a template, the themes and every file it reads, or one of them",
		Long: `Show a template: its description, where it is, the themes a data file may pick, the files it
reads (files:), and each of them as written under ==> file <==: first what a data file of it may hold
(schema.json, checks.yaml, computed.yaml) and its example.yaml to start one from, then page.html,
page.css and words.yaml. With a file named after, print that file alone, as written but the byte
order mark at its start. When the output is long, read the files: line first, then the file you
need: floe template show report example.yaml.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(true)
			if err != nil {
				return err
			}
			_, _, themes, _ := k.Names()
			return a.show(k.Templates(), args, "tmpl_unknown", themes)
		},
	}, &cobra.Command{
		Use:   "new <name>",
		Short: "Make a template of your own: the smallest that floe template check takes, to fill in",
		Long: `Make the template <name> in FLOE_TEMPLATES_DIR (else floe/templates under XDG_CONFIG_HOME, or
~/.config): a page.html, a schema.json and an example.yaml, which floe template check takes. Nothing
is written over: with anything of that name there, nothing is made.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.newOwn(kit.KindTemplate, args[0])
		},
	}, &cobra.Command{
		Use:   "check",
		Short: "Check the templates of your own, drawing each one's example.yaml",
		Long: `Read the templates under FLOE_TEMPLATES_DIR (else floe/templates under XDG_CONFIG_HOME,
or ~/.config) with the parts and themes of your own, as floe part check does, then draw the
example.yaml of each template of your own as floe check draws a data file. Every mistake is
printed, and then the warnings of the examples drawn; the exit code is 1 when there is a
mistake, and nothing is printed when they are right and have no warning.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.checkTemplates()
		},
	})
	return cmd
}

func (a *app) themeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theme",
		Short: "List the themes and show one",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the themes (name, where, and an empty description; tab-separated)",
		Long: `List the themes, floe's and your own, one per line, as floe part list does: its name, where it is
(floe, or its directory of your own) and an empty description, as a theme has none, tab-separated.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			a.list(k.Themes())
			return nil
		},
	}, &cobra.Command{
		Use:   "show <name> [file]",
		Short: "Show a theme and every file it reads, or one of them",
		Long: `Show a theme: its name, where it is, the files it reads (files:), and each of them as written
under ==> file <==: palette.css, base.css, then the CSS of each part or look it gives. With a file
named after, print that file alone, as written but the byte order mark at its start. When the output
is long, read the files: line first, then the file you need: floe theme show ink note.css.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			return a.show(k.Themes(), args, "tmpl_unknown_theme", nil)
		},
	})
	return cmd
}

// checkTemplates draws the example.yaml of each template of one's own, with its images under its directory and
// the icons floe check draws with, every mistake told together.
func (a *app) checkTemplates() error {
	k, err := a.kit(true)
	if err != nil {
		return err
	}
	store := a.iconStore()
	var found usererr.List
	for _, name := range k.OwnTemplates() {
		t, _ := k.Template(name)
		file := t.File("example.yaml")
		images := files.NewRoot(t.File("."))
		o := tmpl.Options{Theme: "", Untitled: name, Images: images, ImagesDir: "", Icons: store}
		_, warnings, err := tmpl.Draw(k, file, t.Example(), o)
		a.warnings = append(a.warnings, warnings...)
		_ = images.Close() //nolint:errcheck // only read from
		if failed := found.Add(err); failed != nil {
			return failed
		}
	}
	return found.Err()
}

// list prints each of entries: its name, where it is, and its description on one line.
func (a *app) list(entries []kit.Entry) {
	for _, e := range entries {
		a.print("%s\t%s\t%s", e.Name, where(e), e.About)
	}
}

// show prints the entry of entries named args[0] and its files, or the file args[1] of it alone, as written but a
// byte order mark at its start; or refuses the name by unknown with the names there are. A template's comes with
// the themes a data file may pick.
func (a *app) show(entries []kit.Entry, args []string, unknown string, themes []string) error {
	i := slices.IndexFunc(entries, func(e kit.Entry) bool { return e.Name == args[0] })
	if i < 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		return usererr.New(unknown, args[0], strings.Join(names, ", "))
	}
	e := entries[i]
	files := make([]string, len(e.Files))
	for i, f := range e.Files {
		files[i] = f.Name
	}
	if len(args) == 2 {
		j := slices.Index(files, args[1])
		if j < 0 {
			return usererr.New("cli_unknown_file", e.Name, args[1], strings.Join(files, ", "))
		}
		a.write(e.Files[j].Text)
		return nil
	}
	if e.About == "" {
		a.print("%s", e.Name)
	} else {
		a.print("%s: %s", e.Name, e.About)
	}
	a.print("%s", english.Msg("cli_where", where(e)))
	if len(e.Looks) > 0 {
		a.print("%s", english.Msg("cli_looks", strings.Join(e.Looks, ", ")))
	}
	if themes != nil {
		a.print("%s", english.Msg("cli_themes", strings.Join(themes, ", ")))
	}
	if len(files) > 0 {
		a.print("%s", english.Msg("cli_files", strings.Join(files, ", ")))
	}
	for _, f := range e.Files {
		a.print("==> %s <==", f.Name)
		a.print("%s", strings.TrimSuffix(string(f.Text), "\n"))
	}
	return nil
}

// where is where e is: its directory of one's own, or floe.
func where(e kit.Entry) string {
	if e.Dir == "" {
		return "floe"
	}
	return e.Dir
}
