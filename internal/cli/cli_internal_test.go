package cli

import (
	"io"
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// A command line floe cannot read is the user's mistake, told in cobra's words, not floe's failure.
func TestUsageIsAMistake(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version", "x"}, {"--nope"}} {
		root, _ := newRoot(Env{Stdout: io.Discard, Stderr: io.Discard, Dir: ""}, io.Discard)
		root.SetArgs(args)
		err := root.Execute()
		r := usererr.Tell(english, err)
		if r.Failure != "" || len(r.Mistakes) != 1 || err.Error() != r.Render("") {
			t.Errorf("floe %v told %+v, %q, want one mistake", args, r, err)
		}
	}
}
