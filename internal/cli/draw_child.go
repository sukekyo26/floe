package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"floe/internal/daemon"
	"floe/internal/drawhere"
	"floe/internal/drawproc"
)

// errNoStdin is floe __draw's failure without what to read what to draw from.
var errNoStdin = errors.New("floe __draw reads what to draw on stdin, which it has none of")

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
		return errNoStdin
	}
	build, err := daemon.Build()
	if err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	a.stopCode = drawproc.Stopped
	k, kitErr := a.kit(true)
	icons := a.iconStore()
	exit := a.env.Exit
	if exit == nil {
		// floe run in a test's process, which ends nothing.
		exit = func(int) {}
	}
	//nolint:wrapcheck // what it read or wrote, which says so
	return drawproc.Serve(a.env.Stdin, a.out, build, drawproc.ChildLimit, exit, func(job drawproc.Job) drawproc.Result {
		if kitErr != nil {
			return drawproc.Result{Page: nil, Warnings: nil, Err: kitErr, Read: nil, Places: nil}
		}
		// Watched while it draws alone, as the CLI's commands are: not while its answer is written.
		a.watchDrawing(job.File)
		defer a.drawn()
		return drawhere.One(k, icons, job)
	})
}
