package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "List the templates, show what a data file of one may hold, and make one of your own",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(a.templateListCmd(), a.templateShowCmd(), a.templateNewCmd())
	return cmd
}

func (a *app) templateListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the templates (name, where, themes, the floe it needs, description; tab-separated)",
		Long: `List the templates, one per line: its name, where it is (a directory of your own, or
built-in), its themes, the floe it needs, and its description, tab-separated. A template of
your own takes the place of the built-in one of its name.`,
		Args: usage(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			l := a.templates()
			names, err := l.Names()
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			for _, name := range names {
				d, err := l.Describe(name)
				if err != nil {
					return err //nolint:wrapcheck // already says what failed
				}
				a.out("%s\t%s\t%s\t%s\t%s", d.Name, a.where(d), strings.Join(d.Themes, ","), d.Floe, d.About)
			}
			return nil
		},
	}
}

func (a *app) where(d *tmpl.Description) string {
	if d.Path == "" {
		return a.cat.Msg("cli_template_builtin")
	}
	return d.Path
}

func (a *app) templateShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show the items a data file of a template may hold, and its example",
		Long: `Show a template: where it is, its themes, the floe it needs, the items a data file of it
may hold (from its schema.json), and its example.yaml to start a data file from.`,
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
		head += ": " + d.About
	}
	a.out("%s", head)
	a.out("%s", a.cat.Msg("cli_template_where", a.where(d)))
	a.out("%s", a.cat.Msg("cli_template_themes", strings.Join(d.Themes, ", ")))
	if d.Floe != "" {
		a.out("%s", a.cat.Msg("cli_template_floe", d.Floe, d.Series))
	}
	a.out("")
	a.out("%s", a.cat.Msg("cli_template_items"))
	a.showItems(d.Items, "  ")
	if d.Example != nil {
		a.out("")
		a.out("%s", a.cat.Msg("cli_template_example"))
		a.out("%s", strings.TrimRight(string(d.Example), "\n"))
	}
}

// showItems writes each item on a line, its name, what it takes and its description, and the items
// under it indented.
func (a *app) showItems(items []tmpl.Item, indent string) {
	for _, it := range items {
		what := a.types(it.Types)
		if len(it.Of) > 0 {
			what = a.cat.Msg("cli_template_list_of", a.types(it.Of))
		}
		parts := []string{}
		if what != "" {
			parts = append(parts, what)
		}
		if it.Format != "" {
			parts = append(parts, a.cat.Msg("cli_template_format", it.Format))
		}
		if len(it.Enum) > 0 {
			parts = append(parts, a.cat.Msg("cli_template_enum", strings.Join(it.Enum, ", ")))
		}
		if it.Required {
			parts = append(parts, a.cat.Msg("cli_template_required"))
		}
		line := indent + it.Name + ": " + strings.Join(parts, a.cat.Msg("cli_template_and"))
		if it.Description != "" {
			line += " — " + it.Description
		}
		a.out("%s", line)
		a.showItems(it.Items, indent+"  ")
	}
}

// types words JSON types as alternatives: text or a number.
func (a *app) types(types []string) string {
	words := make([]string, len(types))
	for i, t := range types {
		words[i] = a.cat.Msg("tmpl_type_" + t)
	}
	return strings.Join(words, a.cat.Msg("tmpl_or"))
}

func (a *app) templateNewCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "new <name> --from <template>",
		Short: "Make a template of your own as a copy of another, to edit",
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
