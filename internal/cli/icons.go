package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/icons"
)

func (a *app) iconsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "Keep icon sets for the icon parts of diagrams, and search them",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	cmd.AddCommand(a.iconsFetchCmd(), a.iconsSearchCmd())
	return cmd
}

func (a *app) iconsFetchCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "fetch [zip-or-url]",
		Short: "Keep the AWS icons of the official Icon package for icon parts",
		Long: `Keep the AWS Architecture Icons (48px services and resources, 32px groups, with their
variants for dark backgrounds) in $FLOE_ICONS_DIR/aws for the icon parts of diagrams.
Without an argument, the Icon package zip (about 14MB) is found on its official page; a
downloaded zip or its URL can be given instead.
The icons are replaced as a whole, and the previous ones stay if that fails. A version
already there is kept unless --force.`,
		Args: usage(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := ""
			if len(args) == 1 {
				source = args[0]
				if !icons.IsURL(source) && !filepath.IsAbs(source) {
					source = filepath.Join(a.env.Dir, source)
				}
			}
			got, err := icons.New(a.cfg.icons).Fetch(cmd.Context(), icons.Default, source, force)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			if got.Already {
				a.say("icons_already", got.Title, got.Version, got.Dir)
			} else {
				a.say("icons_fetched", got.Count, got.Title, got.Version, got.Dir)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "fetch the icons again even if that version is already there")
	return cmd
}

func (a *app) iconsSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <word>...",
		Short: "List the fetched icons whose name contains every word",
		Long: `List the fetched icons whose name contains every word, one per line as the name to give
an icon part (aws/…) and its category, tab-separated.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			hits, err := icons.New(a.cfg.icons).Search(args)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			for _, h := range hits {
				a.out("%s\t%s", h.Name, h.Category)
			}
			return nil
		},
	}
}
