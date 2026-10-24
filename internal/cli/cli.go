// Package cli builds and runs the floe command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
)

// Exit codes.
const (
	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
)

// Env is what floe takes from its process.
type Env struct {
	Getenv func(string) string
	// Dir is the working directory: it resolves relative files and names the project of a page.
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
	// Environ is handed to a server that a publish starts.
	Environ []string
}

// usageError is a command line floe cannot read, or that lacks what it needs (exit 2). A value floe refuses
// is not one (exit 1), whether the command line or the server tells it.
type usageError struct {
	err error
}

func (e usageError) Error() string {
	return e.err.Error()
}

func (e usageError) Unwrap() error {
	return e.err
}

// Run executes floe with args and returns the process exit code. The exit code is decided here only.
func Run(ctx context.Context, args []string, env Env) int {
	root := newRoot(env) //nolint:contextcheck // cobra hands ctx to the commands through ExecuteContext
	root.SetArgs(args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}
	// SilenceErrors leaves printing to this single boundary, which tells err in English (the server's
	// report as it sent it) and lays it out as every mistake is: those at a place as they are, one per
	// line, for editors and CI to read, floe's name before the others.
	_, _ = fmt.Fprintln(env.Stderr, usererr.Tell(english, err).Render(named)) //nolint:errcheck // nowhere left to report
	if errors.As(err, new(usageError)) {
		return exitUsage
	}
	return exitFail
}

// checkLang refuses a --lang that is not a BCP 47 tag.
func checkLang(lang string) error {
	if !i18n.IsTag(lang) {
		return usererr.New("lang_invalid", lang)
	}
	return nil
}

// usage marks the errors of an argument validator as usage errors.
func usage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return usageError{err: err}
		}
		return nil
	}
}

// english is the language of the command line, whatever the environment's: an agent reads it.
var english = i18n.EnglishCatalog()

// named is floe's name before each mistake the command line tells that is not at a place, and its failure.
const named = "floe: "

func newRoot(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "floe",
		Annotations:   uses(nil),
		Short:         "Serve agent-written HTML pages on a local server",
		Version:       version.Version,
		Args:          usage(cobra.NoArgs),
		SilenceErrors: true,
		SilenceUsage:  true,
		// Without a RunE cobra treats the root as non-runnable and answers an
		// unknown subcommand with help and exit 0 instead of NoArgs' error.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // a stdout write error needs no extra context
		},
	}
	root.SetVersionTemplate("floe {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err: err}
	})
	root.AddCommand(&cobra.Command{
		Use:         "version",
		Annotations: uses(nil),
		Short:       "Print the floe version",
		Args:        usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "floe %s\n", version.Version)
			return err //nolint:wrapcheck // a stdout write error needs no extra context
		},
	})
	app := &app{env: env, cfg: nil, lib: nil}
	// The environment is read before any command runs; each command is told the mistakes of the settings it
	// uses, as it declares them.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		app.cfg = loadConfig(env.Getenv, env.Dir)
		return app.settle(cmd)
	}
	root.AddCommand(app.publishCmd(), app.checkCmd(), app.templateCmd(), app.partCmd(), app.listCmd(), app.rmCmd(),
		app.serveCmd(), app.stopCmd(), app.diagramCmd(), app.iconsCmd(), app.renderCmd())
	return root
}

// arg is a path a command was given: its name as it was written, which its mistakes and its page go by, and
// where it is, or why it is nowhere.
type arg struct {
	name, abs string
	err       error // a relative path under no working directory; abs is then ""
}

// arg resolves path for the command, once: under the working directory (dirs.Under), folding its .. by its
// letters, so that every place reads a path alike; a relative one is refused when the working directory
// cannot be read (main's Getwd failed: Dir ""). "" is refused as empty, by every command alike: a shell
// variable left unset gives it, and it names no file. A name of spaces alone is a file of that name.
func (a *app) arg(path string) arg {
	if path == "" {
		return arg{name: path, abs: "", err: usererr.New("file_path_empty")}
	}
	abs, ok := dirs.Under(a.env.Dir, path)
	if !ok {
		return arg{name: path, abs: "", err: usererr.New("cli_relative_no_cwd", path)}
	}
	return arg{name: path, abs: abs, err: nil}
}

// args is the arg of each of the files or directories given, in order, each once (once).
func (a *app) args(paths []string) []arg {
	given := make([]arg, len(paths))
	for i, path := range paths {
		given[i] = a.arg(path)
	}
	return once(given)
}

// once is the args given, in order, each once, by the path it was first given as: two paths that name one
// entry of one directory (./a.yaml and a.yaml, through a symlinked directory, in another case on macOS) are
// read and told once. A link of another name or in another directory, symbolic or hard, is one of its own,
// as its name names its page or template.
// LIMIT: each file is compared with each kept; enough for the files of a command line.
func once(given []arg) []arg {
	var named []arg
	var kept []entry
next:
	for _, p := range given {
		e := entryOf(p)
		for _, k := range kept {
			if e.same(k) {
				continue next
			}
		}
		named, kept = append(named, p), append(kept, e)
	}
	return named
}

// read reads the file p, or tells why it cannot.
func (p arg) read() ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	src, err := os.ReadFile(p.abs) //nolint:gosec // G304: reading the file the user names is the point
	if err != nil {
		return nil, usererr.CannotRead(p.name, err)
	}
	return src, nil
}

// entry is a path as a name in a directory: its absolute path, its directory and itself, nil when they
// cannot be read.
type entry struct {
	abs       string
	dir, info os.FileInfo
}

func entryOf(p arg) entry {
	if p.err != nil || p.abs == "" {
		// Nowhere, or "", by its name: the command refuses it, and nothing is looked up for it.
		return entry{abs: p.name, dir: nil, info: nil}
	}
	e := entry{abs: p.abs, dir: nil, info: nil}
	if dir, err := os.Stat(filepath.Dir(e.abs)); err == nil {
		e.dir = dir
	}
	if info, err := os.Lstat(e.abs); err == nil {
		e.info = info
	}
	return e
}

// same reports whether e and o name one entry of one directory: one directory, and one name, or names that
// differ only in case for one file (on a file system that ignores case). Without a directory, one path.
// LIMIT: on a case-sensitive file system, two hard links in one directory whose names differ only in case are one.
func (e entry) same(o entry) bool {
	if e.dir == nil || o.dir == nil {
		return e.abs == o.abs
	}
	x, y := filepath.Base(e.abs), filepath.Base(o.abs)
	return os.SameFile(e.dir, o.dir) &&
		(x == y || strings.EqualFold(x, y) && e.info != nil && o.info != nil && os.SameFile(e.info, o.info))
}
