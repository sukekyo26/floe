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
		{[]string{"version", "x"}, 2, "", "floe: unknown command \"x\" for \"floe version\": run floe --help\n"},
		{[]string{"nope"}, 2, "", "floe: unknown command \"nope\" for \"floe\": run floe --help\n"},
		{[]string{"part", "nope"}, 2, "", "floe: unknown command \"nope\" for \"floe part\": run floe --help\n"},
		{[]string{"template", "nope"}, 2, "", "floe: unknown command \"nope\" for \"floe template\": run floe --help\n"},
		{[]string{"icons", "nope"}, 2, "", "floe: unknown command \"nope\" for \"floe icons\": run floe --help\n"},
		{[]string{"diagram", "nope"}, 2, "", "floe: unknown command \"nope\" for \"floe diagram\": run floe --help\n"},
		{[]string{"--nope"}, 2, "", "floe: unknown flag: --nope: run floe --help\n"},
		{[]string{"--\xff"}, 2, "", "floe: unknown flag: --\ufffd: run floe --help\n"},
	} {
		var stdout, stderr bytes.Buffer
		code := cli.Run(c.args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: "", Getenv: nil})
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
	if code := cli.Run(nil, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: "", Getenv: nil}); code != 0 || stderr.Len() != 0 {
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
	if code := cli.Run([]string{"version"}, cli.Env{Stdout: failing{}, Stderr: &stderr, Dir: "", Getenv: nil}); code != 1 ||
		stderr.String() != "floe: disk full\n" {
		t.Errorf("floe version to a failing stdout = %d, stderr %q", code, stderr.String())
	}
}

// run runs floe with args in dir, and returns its exit code, stdout and stderr.
func run(dir string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir, Getenv: nil})
	return code, stdout.String(), stderr.String()
}

// A command line asked help for that floe cannot read is a usage error, help or not; help floe cannot write is
// a failure. Help asked rightly is as cobra writes it.
func TestHelp(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		args []string
		errs string
	}{
		{[]string{"help", "nope"}, `floe: unknown command "nope" for "floe": run floe --help`},
		{[]string{"help", "version", "x"}, `floe: unknown command "x" for "floe version": run floe --help`},
		{[]string{"nope", "--help"}, `floe: unknown command "nope" for "floe": run floe --help`},
		{[]string{"help", "nope", "--help"}, `floe: unknown command "nope" for "floe": run floe --help`},
		{[]string{"completion", "nope"}, `floe: unknown command "nope" for "floe completion": run floe --help`},
		{[]string{"completion", "bash", "x"}, `floe: unknown command "x" for "floe completion bash": run floe --help`},
		{[]string{"nope", "-v"}, `floe: unknown command "nope" for "floe": run floe --help`},
		{[]string{"-v", "nope"}, `floe: unknown command "nope" for "floe": run floe --help`},
		{[]string{"--version", "x"}, `floe: unknown command "x" for "floe": run floe --help`},
		{[]string{"help", "help", "x"}, `floe: unknown command "x" for "floe": run floe --help`},
	} {
		if code, stdout, stderr := run("", c.args...); code != 2 || stdout != "" || stderr != c.errs+"\n" {
			t.Errorf("floe %v = %d, stdout %q, stderr %q; want 2, %q", c.args, code, stdout, stderr, c.errs)
		}
	}

	var stderr bytes.Buffer
	if code := cli.Run([]string{"--help"}, cli.Env{Stdout: failing{}, Stderr: &stderr, Dir: "", Getenv: nil}); code != 1 ||
		stderr.String() != "floe: disk full\n" {
		t.Errorf("floe --help to a failing stdout = %d, stderr %q", code, stderr.String())
	}

	// The help command writes what --help writes, the flags too.
	for _, same := range [][2][]string{
		{{"help"}, {"--help"}},
		{{"help", "version"}, {"version", "--help"}},
		{{"help", "render"}, {"render", "--help"}},
		{{"--help", "render"}, {"render", "--help"}},
		{{"-h", "render"}, {"render", "--help"}},
	} {
		code, got, errs := run("", same[0]...)
		_, want, _ := run("", same[1]...)
		if code != 0 || errs != "" || got != want || !strings.Contains(got, "Flags:") {
			t.Errorf("floe %v = %d, %q, stderr %q; want floe %v's %q", same[0], code, got, errs, same[1], want)
		}
	}
	if code, got, _ := run("", "help", "help"); code != 0 || !strings.HasPrefix(got, "Help provides help for any command in the application.\n"+
		"Simply type floe help [path to command] for full details.\n\nUsage:\n  floe help [command] [flags]\n") {
		t.Errorf("floe help help = %d, %q", code, got)
	}
	want := "check\tCheck data files as their pages are drawn, without writing them\n" +
		"completion\tGenerate the autocompletion script for the specified shell\n" +
		"diagram\tDraw diagrams from specs of parts on a canvas\n" +
		"help\tHelp about any command\n" +
		"icons\tKeep icon sets for the icon parts of diagrams, and search them\n" +
		"list\tList pages, newest first (updated, slug, title, linked path; tab-separated)\n" +
		"part\tList the parts, show one, make one of your own, and check yours\n" +
		"publish\tStore a page drawn from a data file (or --link an HTML file in place) and print its URL\n" +
		"rebuild\tDraw every stored page again with the templates, parts and themes of now\n" +
		"render\tWrite the page of each data file to an HTML file beside it, or in --out-dir\n" +
		"rm\tDelete pages in order, stopping at the first failure (a linked file itself is left alone)\n" +
		"serve\tRun the server in the foreground\n" +
		"stop\tStop the background server\n" +
		"template\tList the templates, show one, make one of your own, and check yours\n" +
		"version\tPrint the floe version\n:4\n"
	if code, got, _ := run("", "__complete", "help", ""); code != 0 || !strings.HasPrefix(got, want) {
		t.Errorf("floe __complete help \"\" = %d, %q, want %q", code, got, want)
	}
	for _, args := range [][]string{{"version"}, {"-v"}, {"completion", "bash"}, {"--help", "render"}, {"-h", "render"}} {
		if code, _, errs := run("", args...); code != 0 || errs != "" {
			t.Errorf("floe %v = %d, stderr %q", args, code, errs)
		}
	}
}
