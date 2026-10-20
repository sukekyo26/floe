package cli

import (
	"cmp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
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
	cmd.AddCommand(a.partListCmd(), a.partShowCmd(), a.partNewCmd())
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

func (a *app) partNewCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "new <name> --from <part>",
		Short: "Make a part of your own as a copy of another, to edit",
		Long: `Make a part of your own in $FLOE_PARTS_DIR/<name> as a copy of another part, floe's or your
own, to edit, and copy the file of that part of each parts theme that has one into
$FLOE_PARTS_DIR/themes/<theme>/<name>.css. The copy's part.html names it as its box. Nothing is
written over: with a part of that name, or a file of a parts theme for it, nothing is made. A copy
named like one of floe's parts takes its place.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if from == "" {
				return usageError{err: usererr.New("cli_part_no_from")}
			}
			made, err := a.templates().NewPart(args[0], from)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			original := from
			if made.Builtin {
				original = a.cat.Msg("cli_part_floes", from)
			}
			a.say("cli_part_made", args[0], made.At, original)
			if made.Replaces {
				a.say("cli_part_replaces", args[0])
			}
			if made.Themes != nil {
				a.say("cli_part_themes_copied", strings.Join(made.Themes, ", "), made.ThemesAt, args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "the part to copy (required)")
	return cmd
}
