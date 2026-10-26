package cli

import (
	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) partCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "part",
		Short: "Check the parts, looks and themes of your own",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(&cobra.Command{
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
		Short: "Check the templates of your own",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(&cobra.Command{
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

// checkTemplates draws the example.yaml of each template of one's own, with its images under its directory,
// every mistake told together.
func (a *app) checkTemplates() error {
	k, err := a.kit(true)
	if err != nil {
		return err
	}
	var found usererr.List
	for _, name := range k.OwnTemplates() {
		t, _ := k.Template(name)
		file := t.File("example.yaml")
		_, err := tmpl.Draw(k, file, t.Example(), tmpl.Options{Theme: "", Untitled: name, ImagesDir: t.File(".")})
		if failed := found.Add(err); failed != nil {
			return failed
		}
	}
	return found.Err()
}
