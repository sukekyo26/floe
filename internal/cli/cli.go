// Package cli builds and runs the floe command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

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

// usageError is a mistake in the command line (exit 2).
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
	// SilenceErrors leaves printing to this single boundary, which words a usererr in English.
	shown := usererr.Show(english, err)
	// Placed mistakes go out as they are, one per line, for editors and CI to read, and so do those the
	// server placed.
	prefix := "floe: "
	var remote *remoteError
	if shown.Placed || errors.As(err, &remote) && remote.mistakes {
		prefix = ""
	}
	_, _ = fmt.Fprintf(env.Stderr, "%s%s\n", prefix, shown.Text) //nolint:errcheck // nowhere left to report
	if errors.As(err, new(usageError)) {
		return exitUsage
	}
	return exitFail
}

// checkLang refuses a --lang that is not a BCP 47 tag, as a usage error.
func checkLang(lang string) error {
	if !i18n.IsTag(lang) {
		return usageError{err: usererr.New("lang_invalid", lang)}
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

func newRoot(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "floe",
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
		Use:   "version",
		Short: "Print the floe version",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "floe %s\n", version.Version)
			return err //nolint:wrapcheck // a stdout write error needs no extra context
		},
	})
	app := &app{env: env, cfg: nil}
	// The environment is checked before any command runs, so a typo fails the same way everywhere.
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		cfg, err := loadConfig(env.Getenv, env.Dir)
		app.cfg = cfg
		return err
	}
	root.AddCommand(app.publishCmd(), app.checkCmd(), app.templateCmd(), app.partCmd(), app.listCmd(), app.rmCmd(),
		app.serveCmd(), app.stopCmd(), app.diagramCmd(), app.iconsCmd(), app.renderCmd())
	return root
}

// readArg reads a file named on the command line, relative to the working directory.
func (a *app) readArg(file string) ([]byte, error) {
	src, err := os.ReadFile(a.abs(file)) //nolint:gosec // G304: reading the file the user names is the point
	if err != nil {
		return nil, usererr.CannotRead(file, err)
	}
	return src, nil
}
