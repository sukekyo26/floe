// Package cli builds and runs the floe command tree.
package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"

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
	Stdout io.Writer
	Stderr io.Writer
	Dir    string                  // the working directory a relative path is under; "" when it cannot be read
	Getenv func(key string) string // the environment; nil for none
}

// usageError is a command line floe cannot read, or that lacks what it needs (exit 2).
type usageError struct {
	err *usererr.Error
}

func (e usageError) Error() string {
	return e.err.Error()
}

func (e usageError) Unwrap() error {
	return e.err
}

// usageOf is the usage error of err, cobra's words for a command line it cannot read, with where to look.
func usageOf(err error) usageError {
	return usageError{err: usererr.LibraryWith("cli_usage", err)}
}

// Run executes floe with args and returns the process exit code. The exit code is decided here only.
func Run(args []string, env Env) int {
	out := &written{w: env.Stdout, err: nil}
	root := newRoot(env, out)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(env.Stderr)
	err := readable(root, args)
	if err == nil {
		err = root.Execute()
	}
	if err == nil && out.err != nil {
		err = out.err
	}
	if err == nil {
		return exitOK
	}
	// SilenceErrors leaves printing to this single boundary, which tells err in English.
	_, _ = fmt.Fprintln(env.Stderr, usererr.Tell(english, err).Render(named)) //nolint:errcheck // nowhere left to report
	if errors.As(err, new(usageError)) {
		return exitUsage
	}
	return exitFail
}

// written is a writer that keeps the first error of writing to w, which cobra's help does not return.
type written struct {
	w   io.Writer
	err error
}

func (o *written) Write(p []byte) (int, error) {
	n, err := o.w.Write(p)
	if err != nil && o.err == nil {
		o.err = err
	}
	return n, err //nolint:wrapcheck // kept as it is, for Run to tell
}

// usage marks the errors of an argument validator as usage errors, once.
func usage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := validate(cmd, args)
		if err == nil || errors.As(err, new(usageError)) {
			return err
		}
		return usageOf(err)
	}
}

// english is the language of the command line, whatever the environment's: an agent reads it.
var english = i18n.EnglishCatalog()

// named is floe's name before each mistake the command line tells.
const named = "floe: "

// newRoot returns the command tree, writing to out.
func newRoot(env Env, out io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "floe",
		Short:         "Serve agent-written HTML pages on a local server",
		Version:       version.Version,
		Args:          cobra.NoArgs,
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
		return usageOf(err)
	})
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the floe version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "floe %s\n", version.Version)
			return err //nolint:wrapcheck // a stdout write error needs no extra context
		},
	})
	a := &app{env: env, out: out}
	root.AddCommand(a.checkCmd(), a.iconsCmd(), a.renderCmd())
	// cobra adds these as it runs: here, so that readable finds them, and their arguments are wrapped below.
	root.InitDefaultHelpCmd()
	// cobra's help takes any words, and answers one that names no command with root's help: its words are a
	// command of root.
	for _, c := range root.Commands() {
		if c.Name() == "help" {
			c.Args = func(_ *cobra.Command, args []string) error { return topic(root, args) }
		}
	}
	root.InitDefaultCompletionCmd()
	// Every command's arguments are the user's to fix: cobra's completion command's too. Its --help (and root's
	// --version) are there before readable finds a command: cobra adds them as it runs the one it found, and
	// finds the command past a flag it has not, as one taking the next word (floe --help render).
	var wrap func(c *cobra.Command)
	wrap = func(c *cobra.Command) {
		if c.Args != nil {
			c.Args = usage(c.Args)
		}
		c.InitDefaultHelpFlag()
		c.InitDefaultVersionFlag()
		for _, sub := range c.Commands() {
			wrap(sub)
		}
	}
	wrap(root)
	return root
}

// readable refuses args, a command line, that floe cannot read, before cobra runs it: cobra answers --help
// and --version, and a command that runs nothing with its help, before it checks the arguments and with no
// error, so that a misspelt command would end well. Its flags are read, and such a command is refused the
// arguments it does not take (help takes the topics it has). One that runs is checked by cobra as it runs
// it. cobra's own request for completions, which it adds as it runs, is its own command line, not one to
// refuse.
func readable(root *cobra.Command, args []string) error {
	if len(args) > 0 && slices.Contains([]string{cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd}, args[0]) {
		return nil
	}
	cmd, rest, err := root.Find(args)
	if err != nil {
		return usageOf(err)
	}
	// LIMIT: cobra reads the flags again as it runs; a flag that counts or gathers (a slice) would count twice.
	if err := cmd.ParseFlags(rest); err != nil {
		return usageOf(err)
	}
	// A command that runs nothing is answered with its help too.
	asked := !cmd.Runnable()
	for _, flag := range []string{"help", "version"} {
		if f := cmd.Flags().Lookup(flag); f != nil && f.Changed {
			asked = true
		}
	}
	if !asked {
		return nil
	}
	return given(cmd, cmd.Flags().Args())
}

// topic refuses args, the words the help command was given, that name no command of root.
func topic(root *cobra.Command, args []string) error {
	cmd, rest, err := root.Find(args)
	if err != nil {
		return usageOf(err)
	}
	return given(cmd, rest)
}

// given refuses args, written after cmd asked for its help, that cmd does not take: a misspelt command. Help
// is asked of a command without the arguments it needs.
func given(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return cmd.ValidateArgs(args) //nolint:wrapcheck // a usage error already
}
