package cli

import (
	"cmp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "template",
		Annotations: uses(nil),
		Short:       "List the templates, show what a data file of one may hold, make one of your own, and check it",
		Args:        usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(a.templateListCmd(), a.templateShowCmd(), a.templateNewCmd(), a.templateCheckCmd())
	return cmd
}

func (a *app) templateListCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Annotations: uses(map[setting]use{templatesSetting: reads, partsSetting: reads, themesSetting: reads}),
		Short:       "List the templates (name, where, the floe it needs, description; tab-separated)",
		Long: `List the templates, one per line: its name, where it is (a directory of your own, or
built-in), the floe it needs, and its description, tab-separated. A template of
your own takes the place of the built-in one of its name.`,
		Args: usage(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			l := a.templates()
			names, err := l.Names()
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			// A broken template does not hide the others: it is reported after them (exit code 1).
			var broken usererr.List
			for _, name := range names {
				d, err := l.Describe(name)
				if err != nil {
					tell(&broken, err)
					continue
				}
				// The columns are for scripts to read, in every language: built-in is not translated.
				where := cmp.Or(d.Path, "built-in")
				a.out("%s\t%s\t%s\t%s", d.Name, oneLine(where), d.Floe, oneLine(d.About))
			}
			return broken.Err()
		},
	}
}

// oneLine keeps a column of a line to itself: tabs and line ends in it become spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func where(d *tmpl.Description) string {
	if d.Path == "" {
		return english.Msg("cli_template_builtin")
	}
	return d.Path
}

func (a *app) templateShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "show <name>",
		Annotations: uses(map[setting]use{templatesSetting: reads, partsSetting: reads, themesSetting: reads}),
		Short:       "Show the items a data file of a template may hold, and its example",
		Long: `Show a template: where it is, the themes a data file of it may pick, the floe it needs, the items a
data file of it may hold (from its schema.json), and its example.yaml to start a data file from.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			d, err := a.templates().Describe(args[0])
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			a.showTemplate(d)
			return nil
		},
	}
}

func (a *app) showTemplate(d *tmpl.Description) {
	head := d.Name
	if d.About != "" {
		head += ": " + oneLine(d.About)
	}
	a.out("%s", head)
	a.out("%s", english.Msg("cli_template_where", where(d)))
	a.out("%s", english.Msg("cli_template_themes", strings.Join(d.Themes, ", ")))
	if d.Floe != "" {
		a.out("%s", english.Msg("cli_template_floe", d.Floe, d.Series))
	}
	a.out("")
	a.out("%s", english.Msg("cli_template_items"))
	items := append(sharedItems(d.Name), d.Items...)
	slices.SortFunc(items, func(x, y tmpl.Item) int { return cmp.Compare(x.Name, y.Name) })
	a.showItems(items, "  ")
	if d.Example != nil {
		a.out("")
		a.out("%s", english.Msg("cli_template_example"))
		a.out("%s", strings.TrimRight(string(d.Example), "\n"))
	}
}

// sharedItems are the keys floe reads from every data file of the template name itself, with their
// descriptions.
func sharedItems(name string) []tmpl.Item {
	items := tmpl.SharedItems(name)
	for i, it := range items {
		var args []any
		if it.Name == "theme" {
			args = []any{tmpl.DefaultTheme}
		}
		items[i].Description = english.Msg("cli_template_shared_"+it.Name, args...)
	}
	return items
}

// showItems writes each item on a line, its name, what it takes and its description, and the items
// under it indented.
func (a *app) showItems(items []tmpl.Item, indent string) {
	for _, it := range items {
		what := takes(it)
		parts := []string{}
		if what != "" {
			parts = append(parts, what)
		}
		if it.Format != "" {
			parts = append(parts, english.Msg("cli_template_format", it.Format))
		}
		switch {
		case it.Unusable:
			parts = append(parts, english.Msg("tmpl_part_broken", it.Part))
		case it.Part != "":
			parts = append(parts, english.Msg("cli_template_part", it.Part))
		}
		if len(it.Enum) > 0 {
			parts = append(parts, english.Msg("cli_template_enum", strings.Join(it.Enum, ", ")))
		}
		if it.Required {
			parts = append(parts, english.Msg("cli_template_required"))
		}
		line := indent + it.Name
		if len(parts) > 0 {
			line += ": " + strings.Join(parts, english.Msg("tmpl_and"))
		}
		if it.Description != "" {
			line += " — " + oneLine(it.Description)
		}
		a.out("%s", line)
		a.showItems(it.Items, indent+"  ")
	}
}

// takes words the types an item takes, a list as a list of what its items take: a list of text or
// nothing.
func takes(it tmpl.Item) string {
	words := make([]string, len(it.Types))
	for i, t := range it.Types {
		words[i] = tmpl.Types([]string{t}).Localize(english)
		if t == "array" && len(it.Of) > 0 {
			words[i] = english.Msg("cli_template_list_of", tmpl.Types(it.Of).Localize(english))
		}
	}
	return strings.Join(words, english.Msg("tmpl_or"))
}

func (a *app) templateNewCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:         "new <name> --from <template>",
		Annotations: uses(map[setting]use{templatesSetting: needs}),
		Short:       "Make a template of your own as a copy of another, to edit",
		Long: `Make a template of your own in $FLOE_TEMPLATES_DIR/<name> as a copy of another template,
built-in or your own, to edit. Its template.yaml names the floe it needs (this floe, for a copy of
a built-in one), and its example.yaml names the new template. A template of your own of that name
is left as it is. A copy named like a built-in template takes its place.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if from == "" {
				return usageError{err: usererr.New("cli_template_no_from")}
			}
			at, err := a.templates().New(args[0], from)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			a.say("cli_template_made", args[0], at, from)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "the template to copy (required)")
	return cmd
}

func (a *app) templateCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "check <dir>...",
		Annotations: uses(map[setting]use{iconsSetting: reads, partsSetting: reads, themesSetting: reads}),
		Short:       "Check templates before you use or share them",
		Long: `Check each directory as a template of its name: its template.yaml, schema.json, checks.yaml,
computed.yaml, page.html, words.yaml, default theme and assets/, and then its example.yaml through every stage of
floe publish, drawing the page included. Every mistake is printed, at its line and column where
it has one, and the exit code is 1 when there is one. Warnings go to stderr and leave the exit
code 0. Nothing is printed when the templates are right.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			// One list, each template's warnings and mistakes in the order given, told once.
			var all usererr.List
			failed := false
			// "" is the working directory, as filepath.Abs took it, and under none it is refused; its name
			// stays "". LIMIT: what "" names is for floe-usage-issues 62 to decide.
			given := make([]arg, len(args))
			for i, dir := range args {
				given[i] = a.arg(cmp.Or(dir, "."))
				given[i].name = dir
			}
			for _, p := range once(given) {
				var warnings usererr.List
				err := p.err
				if err == nil {
					warnings, err = a.templates().Check(p.name, a.env.Dir)
				}
				all = append(all, warnings...)
				told := len(all)
				tell(&all, err)
				failed = failed || len(all) > told
			}
			if failed {
				return all.Err()
			}
			a.tellWarnings(all)
			return nil
		},
	}
}
