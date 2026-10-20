package cli

import (
	"cmp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/tmpl"
)

func (a *app) partCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "part",
		Short: "List the parts, show what one takes, make one of your own, and check yours",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(a.partListCmd(), a.partShowCmd())
	return cmd
}

func (a *app) partListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the parts (name, where, description; tab-separated)",
		Long: `List the parts, floe's and those of your own in $FLOE_PARTS_DIR, one per line: its name,
where it is (a directory of your own, or built-in) and its description, tab-separated. A part of
your own takes the place of floe's of its name.`,
		Args: usage(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			parts, broken, err := a.templates().Parts()
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			for _, d := range parts {
				// The columns are for scripts to read, in every language: built-in is not translated.
				a.out("%s\t%s\t%s", d.Name, oneLine(cmp.Or(d.Path, "built-in")), oneLine(d.About))
			}
			// A part that cannot be used does not hide the others: it is reported after them (exit code 1).
			if broken != nil {
				return mistakes(broken)
			}
			return nil
		},
	}
}

func (a *app) partShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show what a part takes, the looks it takes, the parts it holds and its parts themes",
		Long: `Show a part: where it is, what it takes and its items (from its schema.json), the looks it
takes (from its part.yaml, and those they take in turn), the parts it holds, and the parts themes
with a file of it.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			d, err := a.templates().DescribePart(args[0])
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			a.showPart(d)
			return nil
		},
	}
}

func (a *app) showPart(d *tmpl.PartDescription) {
	head := d.Name
	if d.About != "" {
		head += ": " + oneLine(d.About)
	}
	a.out("%s", head)
	switch {
	case d.Path == "":
		a.say("cli_template_where", a.cat.Msg("cli_template_builtin"))
	case d.Replaces:
		a.say("cli_part_where_replaces", d.Path, d.Name)
	default:
		a.say("cli_template_where", d.Path)
	}
	if !d.Look {
		a.say("cli_part_takes", a.takes(d.Takes))
	}
	for _, line := range []struct {
		key   string
		names []string
	}{{"cli_part_looks", d.Looks}, {"cli_part_holds", d.Holds}, {"cli_part_themes", d.Themes}} {
		if len(line.names) > 0 {
			a.say(line.key, strings.Join(line.names, ", "))
		}
	}
	if len(d.Takes.Items) > 0 {
		a.say("cli_template_items")
		a.showItems(d.Takes.Items, "  ")
	}
}
