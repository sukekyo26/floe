package cli

import (
	"fmt"

	"floe/internal/guard"
	"floe/internal/usererr"
)

// drawLimit is the memory the commands that draw may hold: guard.Limit, lower in a test.
var drawLimit uint64 = guard.Limit

// watchDrawing tells the watch of the memory that the command draws file next, starting it at the first: the commands
// that draw call it before each page or data file, and the others never start it.
func (a *app) watchDrawing(file string) {
	if a.env.Exit == nil {
		return
	}
	if a.guard == nil {
		a.guard = guard.Start(drawLimit, a.stopDrawing)
	}
	a.guard.Drawing(file)
}

// stopDrawing tells, at file, that drawing it took more memory than floe draws a page with, and ends the process.
// LIMIT: the mistakes of the files drawn before it are not told.
func (a *app) stopDrawing(file string) {
	limit := fmt.Sprintf("%d MiB", drawLimit>>20)
	err := usererr.New("draw_memory", limit)
	if file != "" {
		err = usererr.NewAt(file, 0, 0, "draw_memory", limit)
	}
	_, _ = fmt.Fprintln(a.env.Stderr, usererr.Tell(english, err).Render(named)) //nolint:errcheck // nowhere left to report
	a.env.Exit(exitFail)
}
