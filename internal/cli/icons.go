package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"floe/internal/diagram"
	"floe/internal/icons"
	"floe/internal/usererr"
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
		Use:   "fetch <set> [zip-or-url...]",
		Short: "Keep an icon set from its official packages for icon parts",
		Long: `Keep an icon set in $FLOE_ICONS_DIR/<set> for the icon parts of diagrams. The sets:

  aws    the AWS Architecture Icons (48px services and resources, 32px groups); the Icon
         package zip is about 14MB
  gcp    the Google Cloud product icons: the core products' own and the product categories',
         which every other product takes; two zips, about 1MB
  azure  the Azure architecture icons; their page lets the zip be downloaded only after
         agreeing to its terms, so floe does not download it: pass the zip or its URL

lucide, every icon of Lucide 1.72.0 (lucide/users, lucide/database), is built into floe and
never fetched: search it with floe icons search lucide <word>.

Without a zip, the set's packages are found on its official page; downloaded zips or their
URLs can be given instead, one for each package of the set (gcp takes both of its zips).
The icons are replaced as a whole, and the previous ones stay if that fails. A version
already there is kept unless --force.

FLOE_ICONS_DIR is an absolute path; without it, the icons are kept in
$XDG_DATA_HOME/floe-icons, or ~/.local/share/floe-icons.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			from := make([]icons.Source, len(args)-1)
			errs := []error{icons.CheckFetch(args[0], len(from))}
			for i, arg := range args[1:] {
				from[i] = icons.Source{Name: arg, At: arg}
				if !icons.IsURL(arg) {
					p := a.arg(arg, place{name: fmt.Sprintf("argument %d", i+2), wants: "cli_wants_icons"})
					from[i].At = p.abs
					errs = append(errs, p.err)
				}
			}
			at, dirErr := a.iconsDirFor(args[0])
			if err := usererr.Collect(append(errs, dirErr)...); err != nil {
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
		Use:   "search <set> <word>...",
		Short: "List the icons of a set, lucide or one fetched, whose name holds every word, or a short name such as alb",
		Long: `List the icons of the set (lucide, aws, gcp, azure) whose name contains every word, one per line as
the name to give an icon part (lucide/…, aws/…, gcp/…, azure/…) and its category, tab-separated. lucide is
built into floe: every icon of Lucide 1.72.0, drawn in the text colour or a tone, which needs no fetch.
An icon whose name is a word whole (user for lucide/user) comes first, as does a common short name of a
service in the set (alb, s3, gke, aks, vnet), or of a thing in lucide (db, person, browser), then the
service's resources. A set not fetched is told with how to fetch it.

  floe icons search gcp storage`,
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			var set string
			if len(args) > 0 {
				set = args[0]
			}
			at, dirErr := a.iconsDirFor(set)
			if err := usererr.Collect(icons.CheckSearch(args), dirErr); err != nil {
				return err //nolint:wrapcheck // mistakes for the user as they are
			}
			hits, err := iconsIn(at).Search(args[0], args[1:])
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
		err = usererr.NewWhole("icons_no_dir")
	}
	return p, err
}

// iconsDirFor is the directory of the icon sets for a command on set: a set built in needs none, so that the
// directory's mistake is not told for it.
func (a *app) iconsDirFor(set string) (dataPlace, error) {
	p, err := a.iconsDir()
	if icons.IsBuiltin(set) {
		err = nil
	}
	return p, err
}

// iconsIn is the store at p: one whose default cannot be holds no set, and a fetch into it tells why.
func iconsIn(p dataPlace) *icons.Store {
	return icons.New(p.dir, p.unusable != nil, p.setting())
}

// iconStore returns the icons the diagrams of one command show: those of iconsDir, or, without a directory for
// them, those built in, a diagram with another icon told why. Made once for the command: a Store keeps the indexes
// it read.
func (a *app) iconStore() diagram.Icons {
	at, err := a.iconsDir()
	if err != nil {
		return icons.Without(err)
	}
	return iconsIn(at)
}
