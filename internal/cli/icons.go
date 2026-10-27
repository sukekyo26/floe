package cli

import (
	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) iconsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "Keep icon sets for the icon parts of diagrams, and search them",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.iconsFetchCmd(), a.iconsSearchCmd())
	return cmd
}

func (a *app) iconsFetchCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "fetch <set> [zip-or-url]",
		Short: "Keep an icon set from its official package for icon parts",
		Long: `Keep an icon set in $FLOE_ICONS_DIR/<set> for the icon parts of diagrams. The sets:

  aws  the AWS Architecture Icons (48px services and resources, 32px groups); the Icon
       package zip is about 14MB

Without a zip, the package is found on the set's official page; a downloaded zip or its
URL can be given instead.
The icons are replaced as a whole, and the previous ones stay if that fails. A version
already there is kept unless --force.

FLOE_ICONS_DIR is an absolute path; without it, the icons are kept in
$XDG_DATA_HOME/floe-icons, or ~/.local/share/floe-icons.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			from := icons.Source{Name: "", At: ""}
			var fromErr error
			if len(args) == 2 {
				from = icons.Source{Name: args[1], At: args[1]}
				if !icons.IsURL(args[1]) {
					p := a.arg(args[1], place{name: "argument 2", wants: "cli_wants_icons"})
					from.At, fromErr = p.abs, p.err
				}
			}
			at, dirErr := a.iconsDir()
			if err := usererr.Collect(icons.CheckSet(args[0]), fromErr, dirErr); err != nil {
				return err //nolint:wrapcheck // mistakes for the user as they are
			}
			got, err := iconsIn(at).Fetch(cmd.Context(), args[0], from, force)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			if got.Already {
				a.print("%s", english.Msg("icons_already", got.Title, got.Version, got.Dir))
			} else {
				a.print("%s", english.Msg("icons_fetched", got.Count, got.Title, got.Version, got.Dir))
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
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			at, err := a.iconsDir()
			if err != nil {
				return err
			}
			hits, err := iconsIn(at).Search(args)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			for _, h := range hits {
				a.print("%s\t%s", h.Name, h.Category)
			}
			return nil
		},
	}
}

// iconsDir is the directory of the icon sets: FLOE_ICONS_DIR, else floe-icons under the XDG data directory,
// else under ~/.local/share (dataDir). The icon sets need a place, and there is none without one.
func (a *app) iconsDir() (dataPlace, error) {
	p, err := a.dataDir("FLOE_ICONS_DIR", "XDG_DATA_HOME", ".local/share", "floe-icons")
	if err == nil && p.dir == "" {
		err = usererr.New("icons_no_dir")
	}
	return p, err
}

// iconsIn is the store at p: one whose default cannot be holds no set, and a fetch into it tells why.
func iconsIn(p dataPlace) *icons.Store {
	return icons.New(p.dir, p.unusable != nil, p.setting())
}

// iconStore returns the icons the diagrams of one command show: those of iconsDir, or, without a directory for
// them, none, which a diagram with an icon is told. Made once for the command: a Store keeps the indexes it read.
func (a *app) iconStore() diagram.Icons {
	at, err := a.iconsDir()
	if err != nil {
		return diagram.NoIcons{Err: err}
	}
	return iconsIn(at)
}
