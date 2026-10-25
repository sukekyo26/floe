package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/version"
)

// A command line floe cannot read exits 2 with floe's name before cobra's words; a failure to write exits 1.
func TestRun(t *testing.T) {
	t.Parallel()

	v := "floe " + version.Version + "\n"
	for _, c := range []struct {
		args         []string
		code         int
		stdout, errs string
	}{
		{[]string{"version"}, 0, v, ""},
		{[]string{"--version"}, 0, v, ""},
		{[]string{"version", "x"}, 2, "", "floe: unknown command \"x\" for \"floe version\"\n"},
		{[]string{"nope"}, 2, "", "floe: unknown command \"nope\" for \"floe\"\n"},
		{[]string{"--nope"}, 2, "", "floe: unknown flag: --nope\n"},
	} {
		var stdout, stderr bytes.Buffer
		code := cli.Run(c.args, cli.Env{Stdout: &stdout, Stderr: &stderr})
		if code != c.code || stdout.String() != c.stdout || stderr.String() != c.errs {
			t.Errorf("floe %v = %d, stdout %q, stderr %q; want %d, %q, %q",
				c.args, code, stdout.String(), stderr.String(), c.code, c.stdout, c.errs)
		}
	}
}

// floe with no arguments prints its help, which lists the version command.
func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if code := cli.Run(nil, cli.Env{Stdout: &stdout, Stderr: &stderr}); code != 0 || stderr.Len() != 0 {
		t.Fatalf("floe = %d, stderr %q", code, stderr.String())
	}
	if out := stdout.String(); !strings.HasPrefix(out, "Serve agent-written HTML pages on a local server\n") ||
		!strings.Contains(out, "  version     Print the floe version\n") {
		t.Errorf("floe printed %q", out)
	}
}

type failing struct{}

func (failing) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

func TestRunWriteFails(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if code := cli.Run([]string{"version"}, cli.Env{Stdout: failing{}, Stderr: &stderr}); code != 1 ||
		stderr.String() != "floe: disk full\n" {
		t.Errorf("floe version to a failing stdout = %d, stderr %q", code, stderr.String())
	}
}
