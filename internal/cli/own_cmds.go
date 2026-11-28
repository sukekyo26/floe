package cli

import (
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"floe/docs"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/tmpl"
	"floe/internal/usererr"
)

func (a *app) partCmd(h ownHelp) *cobra.Command {
	short := "List the parts, show one, make one of your own, and check yours"
	cmd := &cobra.Command{
		Use:   "part",
		Short: short,
		Long:  short + h.text(".\n\nfloe part guide prints how to make one of your own: its files and what floe checks."),
		Args:  cobra.NoArgs,
	}
	var all bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List the parts a section of floe's templates takes, yours too (name, where, shape, description)",
		Long: h.text(`List the parts a section of floe's templates takes, your own parts too, one per line: its name, where it
is (floe, or its directory of your own), its shape and its description, tab-separated. html takes HTML as
text and has no schema.json: floe part show html any.json prints any.json, which holds the key html. Your
own parts join any, but floe part show any any.json prints floe's any.json as it is written. With --all,
list every part, look and shared schema, floe's and your own, that a schema of your own can name with
$ref, where it is (floe, or its directory of your own).

` + shapeHelp),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			if all {
				a.list(k.Parts())
				return nil
			}
			section, err := k.SectionParts()
			if err != nil {
				return err //nolint:wrapcheck // floe's failure
			}
			a.list(section)
			return nil
		},
	}
	list.Flags().BoolVar(&all, "all", false, "list every part, look and shared schema a $ref can name, your own too")
	cmd.AddCommand(list, &cobra.Command{
		Use:   "show <name> [file]",
		Short: "Show a part, look or shared schema (any.json for html) and the files it reads, or one of them",
		Long: h.text(`Show a part, a look or a shared schema: its description, where it is, the looks it takes and the
files it reads (files:): schema.json, checks.yaml, part.yaml, part.html, part.css and words.yaml of a
part, part.yaml and part.css of a look, <name>.json of a shared schema. html, which has no part of its
own, names any.json, which holds the key html. Your own parts join any, but floe part show any any.json
prints floe's any.json as it is written. With a file named after, print that file alone, as written but
the byte order mark at its start: floe part show code part.html.`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			entries, err := shown(k, args[0])
			if err != nil {
				return err
			}
			return a.show(entries, args, "cli_unknown_part", nil)
		},
	}, &cobra.Command{
		Use:   "new <name>",
		Short: h.text("Make a part of your own: the smallest that floe part check takes, to fill in"),
		Long: "Make the part <name> in " + h.place("parts") + h.text(`: a schema.json, a part.html of its box and an
empty part.css, which floe part check takes. Nothing is written over: with anything of that name there,
nothing is made.`),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.newOwn(ownParts, args[0])
		},
	}, &cobra.Command{
		Use:   "check",
		Short: "Check the parts, looks, shared schemas and themes of your own",
		Long: "Read the parts, looks and shared schemas under " + h.place("parts") + " and the themes under " +
			h.place("themes") + h.text(`, over floe's own, and print every mistake of them. The exit code is 1
when there is one; nothing is printed when they are right. A mistake of one of your own stops
floe check and floe render too. With parts of your own, it compiles floe's templates with them too.
floe theme check does the same: a theme's <part>.css needs its part, so parts and themes are checked
together.`),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.checkPartsThemes()
		},
	})
	cmd.AddCommand(a.docGuideCmd(
		"Print how to make a part, a look or a shared schema of your own: their files and what floe checks", docs.PartGuide))
	return cmd
}

func (a *app) templateCmd(h ownHelp) *cobra.Command {
	short := "List the templates, show one, make one of your own, and check yours"
	cmd := &cobra.Command{
		Use:   "template",
		Short: short,
		Long:  short + h.text(".\n\nfloe template guide prints how to make one of your own: its files and what floe checks."),
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the templates (name, where, shape, description; tab-separated)",
		Long: `List the templates, floe's and your own, one per line: its name, where it is (floe, or its
directory of your own), its shape (the keys of its data file) and its description, tab-separated. One of
your own hides floe's of its name.

` + h.text(shapeHelp),
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
		Short: "Show a template, the themes and the files it reads, or one of them",
		Long: h.text(`Show a template: its description, where it is, the themes a data file may pick and the files it
reads (files:): first what a data file of it may hold (schema.json, checks.yaml, computed.yaml) and
its example.yaml to start one from, then page.html, page.css and words.yaml. With a file named after,
print that file alone, as written but the byte order mark at its start: floe template show report
example.yaml.`),
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
		Short: h.text("Make a template of your own to fill in: the smallest floe template check takes, with a viewport"),
		Long: "Make the template <name> in " + h.place("templates") + h.text(`: a page.html (with a viewport <meta>,
so that a phone draws the page at its width), a schema.json and an example.yaml, which
floe template check takes. Nothing is written over: with anything of that name there, nothing is made.`),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.newOwn(ownTemplates, args[0])
		},
	}, &cobra.Command{
		Use:   "check",
		Short: "Check the templates of your own, drawing each one's example.yaml",
		Long: "Read the templates under " + h.place("templates") + h.text(` with the parts and themes of your own,
as floe part check does, then draw the example.yaml of each template of your own as floe check draws a
data file.`) + ` Every mistake is printed, and then the warnings of the examples drawn; the exit code is 1
when there is a mistake, and nothing is printed when they are right and have no warning.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.checkTemplates()
		},
	})
	cmd.AddCommand(a.docGuideCmd("Print how to make a template of your own: its files and what floe checks",
		docs.TemplateGuide))
	return cmd
}

func (a *app) themeCmd(h ownHelp) *cobra.Command {
	short := "List the themes, show one, make one of your own, and check yours"
	cmd := &cobra.Command{
		Use:   "theme",
		Short: short,
		Long:  short + h.text(".\n\nfloe theme guide prints how to make one of your own: its files and what floe checks."),
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the themes (name, where, shape, description; tab-separated)",
		Long: h.text(`List the themes, floe's and your own, one per line in the same four tab-separated columns as
floe part list: its name, where it is (floe, or its directory of your own), its shape, -, as nothing is
written as a theme's value, and its description, from its theme.yaml (empty without one).`),
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
		Short: "Show a theme and the files it reads, or one of them",
		Long: h.text(`Show a theme: its name and description, where it is and the files it reads (files:): theme.yaml,
palette.css, base.css, markdown.css, then the CSS of each part or look it gives. With a file named after,
print that file alone, as written but the byte order mark at its start: floe theme show ink note.css.`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			k, err := a.kit(false)
			if err != nil {
				return err
			}
			return a.show(k.Themes(), args, "cli_unknown_theme", nil)
		},
	}, &cobra.Command{
		Use:   "new <name>",
		Short: h.text("Make a theme of your own: the smallest that floe theme check takes, to fill in"),
		Long: "Make the theme <name> in " + h.place("themes") + h.text(`: an empty palette.css, which goes over
floe's default palette (floe theme show default palette.css) and which floe theme check takes. Nothing is
written over: with anything of that name there, nothing is made.`),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.newOwn(ownThemes, args[0])
		},
	}, &cobra.Command{
		Use:   "check",
		Short: h.text("Check the parts, looks, shared schemas and themes of your own, as floe part check does"),
		Long: h.text(`Do what floe part check does, with the same output and exit code: a theme's <part>.css needs its
part, so parts and themes are checked together. floe part check --help tells what it reads and prints.`),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.checkPartsThemes()
		},
	})
	if !h.repository {
		cmd.AddCommand(a.themeDefaultCmd())
	}
	cmd.AddCommand(a.docGuideCmd("Print how to make a theme of your own: its files and what floe checks", docs.ThemeGuide))
	return cmd
}

// themeDefaultCmd is floe theme default, which reads the store: none of a repository's.
func (a *app) themeDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Print the default theme chosen in the management page's settings, or nothing when none is",
		Long: `Print the name of the default theme chosen in the management page's settings, which a page takes when
it is first stored and names no theme, or nothing when none is chosen (floe's default then). It reads the store
the other commands send to (FLOE_DIR) without starting the server.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			set, err := a.settings()
			if err != nil {
				return err
			}
			if set.Theme != "" {
				a.print("%s", set.Theme)
			}
			return nil
		},
	}
}

// ownHelp is how the commands of one's own name their places and the commands to run: floe's, of the places the
// settings name, or floe repository's, of the repository's.
type ownHelp struct {
	repository bool
}

// repositoryCommand is a command of floe's that floe repository has one of its own for, as a help names it: not
// a word after floe that names no command (the floe part it is).
var repositoryCommand = regexp.MustCompile(
	`\bfloe ((?:part|template|theme) (?:list|show|new|check|guide)|check|render)\b`,
)

// text is s, a help that names commands of floe's, with floe repository's in their place under it.
func (h ownHelp) text(s string) string {
	if h.repository {
		return repositoryCommand.ReplaceAllString(s, "floe repository $1")
	}
	return s
}

// place is the directory of one's own of the kind, as the help says it.
func (h ownHelp) place(kind string) string {
	if h.repository {
		return ".floe/" + kind + " at the repository's root"
	}
	return "FLOE_" + strings.ToUpper(kind) + "_DIR (else floe/" + kind +
		" under XDG_CONFIG_HOME, or ~/.config; ~ is %USERPROFILE% on Windows)"
}

// checkPartsThemes is floe part check and floe theme check: the parts and themes of one's own, read together, and
// floe's templates compiled again with one's own parts.
func (a *app) checkPartsThemes() error {
	k, err := a.kit(false)
	if err != nil {
		return err
	}
	return k.CompileAgain() //nolint:wrapcheck // floe's failure, which says what failed
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
	defer a.drawn()
	for _, name := range k.OwnTemplates() {
		t, _, err := k.Template(name)
		if err != nil {
			return err //nolint:wrapcheck // floe's failure, which says what failed
		}
		file := t.File("example.yaml")
		images := files.NewRoot(t.File("."))
		o := tmpl.Options{
			Theme: "", Default: "", Untitled: name, Images: images, ImagesDir: "", ImageAt: nil, Icons: store, CutOff: false,
		}
		a.watchDrawing(file)
		_, warnings, err := tmpl.Draw(k, file, t.Example(), o)
		a.warnings = append(a.warnings, warnings...)
		_ = images.Close() //nolint:errcheck // only read from
		if failed := found.Add(err); failed != nil {
			return failed
		}
	}
	return found.Err()
}

// shown is what floe part show finds name in: the parts, looks and shared schemas; for a name none of them has,
// what a section of floe's templates takes too (html), by name, so that a name of neither is refused with both.
func shown(k *kit.Kit, name string) ([]kit.Entry, error) {
	entries := k.Parts()
	named := func(e kit.Entry) bool { return e.Name == name }
	if slices.ContainsFunc(entries, named) {
		return entries, nil
	}
	section, err := k.SectionParts()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's failure
	}
	if i := slices.IndexFunc(section, named); i >= 0 {
		return section[i : i+1], nil
	}
	for _, e := range section {
		if !slices.ContainsFunc(entries, func(p kit.Entry) bool { return p.Name == e.Name }) {
			entries = append(entries, e)
		}
	}
	slices.SortFunc(entries, func(a, b kit.Entry) int { return strings.Compare(a.Name, b.Name) })
	return entries, nil
}

// shapeHelp is what the --help of each list says of the shape column.
const shapeHelp = `The shape is what a value of it is written as: its keys, by name; "[] " before those of a list's
items; "one of " before keys of which exactly one is written; the name of the format (markdown, html)
or of the floe part it is; or -, which the list does not tell: floe part show <name> (template show,
theme show) names in its files: line what to read, floe diagram example <kind> prints an example, and
floe diagram guide says how to write a diagram.`

// list prints each of entries: its name, where it is, its shape, and its description on one line.
func (a *app) list(entries []kit.Entry) {
	for _, e := range entries {
		a.print("%s\t%s\t%s\t%s", e.Name, where(e), e.Shape, e.About)
	}
}

// show prints the entry of entries named args[0] and the names of its files, or the file args[1] of it alone, as
// written but a byte order mark at its start; or refuses the name by unknown with the names there are. A template's
// comes with the themes a data file may pick.
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
	return nil
}

// where is where e is: its directory of one's own, or floe.
func where(e kit.Entry) string {
	if e.Dir == "" {
		return "floe"
	}
	return e.Dir
}
