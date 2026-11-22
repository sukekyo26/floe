package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"floe/internal/daemon"
	"floe/internal/drawproc"
)

// drawChildCmd is floe __draw, which the server runs to draw its pages (drawproc), hidden: no one types it.
func (a *app) drawChildCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__draw",
		Short:  "Draw pages for the server, as it asks on stdin",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.drawChild()
		},
	}
}

// drawChild draws the pages the server asks for on stdin, with one's own parts, themes and templates over floe's and
// the icons, as the server reads them, its memory watched: past the limit, it ends with drawproc.Stopped.
func (a *app) drawChild() error {
	if a.env.Stdin == nil {
		return errors.New("floe __draw reads what to draw on stdin, which it has none of")
	}
	build, err := daemon.Build()
	if err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	a.stopCode = drawproc.Stopped
	k, kitErr := a.kit(true)
	icons := a.iconStore()
	//nolint:wrapcheck // what it read or wrote, which says so
	return drawproc.Serve(a.env.Stdin, a.out, build, func(job drawproc.Job) drawproc.Result {
		if kitErr != nil {
			return drawproc.Result{Page: nil, Warnings: nil, Err: kitErr, Read: nil, Places: nil}
		}
		a.watchDrawing(job.File)
		return drawproc.One(k, icons, job)
	})
}
