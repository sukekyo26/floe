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
		RunE:  showHelp,
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
		Use:   "show <name>",
		Short: "Show a part, look or shared schema, and its schema and checks as written",
		Long: `Show a part, a look or a shared schema: its description, where it is, the looks it takes, and the
files that say what a data file may hold of it, as written (schema.json and checks.yaml of a part,
<name>.json of a shared schema).`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			return a.show(k.Parts(), args[0], "cli_unknown_part", nil)
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
		RunE:  showHelp,
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
		Use:   "show <name>",
		Short: "Show a template: the themes, and its schema, checks and example as written",
		Long: `Show a template: its description, where it is, the themes a data file may pick, and as written
the files that say what a data file of it may hold (schema.json, checks.yaml) and its example.yaml
to start one from.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(true)
			if err != nil {
				return err
			}
			_, _, themes, _ := k.Names()
			return a.show(k.Templates(), args[0], "tmpl_unknown", themes)
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
printed; the exit code is 1 when there is one, and nothing is printed when they are right.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.checkTemplates()
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
		_, err := tmpl.Draw(k, file, t.Example(), o)
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

// show prints the entry of entries named name, or refuses the name by unknown with the names there are; a
// template's with the themes a data file may pick.
func (a *app) show(entries []kit.Entry, name, unknown string, themes []string) error {
	i := slices.IndexFunc(entries, func(e kit.Entry) bool { return e.Name == name })
	if i < 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		return usererr.New(unknown, name, strings.Join(names, ", "))
	}
	e := entries[i]
	a.print("%s: %s", e.Name, e.About)
	a.print("%s", english.Msg("cli_where", where(e)))
	if len(e.Looks) > 0 {
		a.print("%s", english.Msg("cli_looks", strings.Join(e.Looks, ", ")))
	}
	if themes != nil {
		a.print("%s", english.Msg("cli_themes", strings.Join(themes, ", ")))
	}
	for _, f := range e.Files {
		a.print("%s:", f.Name)
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
