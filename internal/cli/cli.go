// Package cli builds and runs the floe command tree.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

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
	Dir    string // the working directory a relative path is under; "" when it cannot be read
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
	return usageError{err: usererr.New("cli_usage", err.Error())}
}

// Run executes floe with args and returns the process exit code. The exit code is decided here only.
func Run(args []string, env Env) int {
	out := &written{w: env.Stdout, err: nil}
	root, helped := newRoot(env, out)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(env.Stderr)
	err := root.Execute()
	if err == nil {
		// cobra's way of help returns no error: a command line it could not read is told so.
		err = *helped
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

// usage marks the errors of an argument validator as usage errors.
func usage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return usageOf(err)
		}
		return nil
	}
}

// english is the language of the command line, whatever the environment's: an agent reads it.
var english = i18n.EnglishCatalog()

// named is floe's name before each mistake the command line tells.
const named = "floe: "

// newRoot returns the command tree, writing to out, and where its help keeps the usage error of a command line
// it was asked for: cobra answers --help, and a command that runs nothing, with help and no error.
func newRoot(env Env, out io.Writer) (*cobra.Command, *error) {
	var helped error
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
	root.AddCommand(a.checkCmd(), a.renderCmd())
	help := helpCmd(root, &helped)
	root.SetHelpCommand(help)
	root.InitDefaultCompletionCmd()
	// Every command's arguments are the user's to fix: cobra's completion command's too.
	var wrap func(c *cobra.Command)
	wrap = func(c *cobra.Command) {
		if c.Args != nil {
			c.Args = usage(c.Args)
		}
		for _, sub := range c.Commands() {
			wrap(sub)
		}
	}
	wrap(root)
	shown := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var err error
		if cmd == help {
			_, err = topic(root, cmd.Flags().Args())
		} else {
			err = given(cmd, cmd.Flags().Args())
		}
		if err != nil {
			helped = err
			return
		}
		shown(cmd, args)
	})
	return root, &helped
}

// topic returns the command of root the help command was asked of by args, or why there is none.
func topic(root *cobra.Command, args []string) (*cobra.Command, error) {
	cmd, rest, err := root.Find(args)
	if err != nil {
		return nil, usageOf(err)
	}
	if err := given(cmd, rest); err != nil {
		return nil, err
	}
	return cmd, nil
}

// given refuses args, written after cmd asked for its help, that cmd does not take: a misspelt command. Help
// is asked of a command without the arguments it needs.
func given(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return cmd.ValidateArgs(args) //nolint:wrapcheck // a usage error already
}

// helpCmd returns the help command of root, as cobra's, which tells a topic it has not as a usage error kept
// in helped.
func helpCmd(root *cobra.Command, helped *error) *cobra.Command {
	var help *cobra.Command
	help = &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type ` + root.DisplayName() + ` help [path to command] for full details.`,
		ValidArgsFunction: func(
			c *cobra.Command, args []string, toComplete string,
		) ([]cobra.Completion, cobra.ShellCompDirective) {
			cmd, _, err := c.Root().Find(args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var completions []cobra.Completion
			for _, sub := range cmd.Commands() {
				if (sub.IsAvailableCommand() || sub == help) && strings.HasPrefix(sub.Name(), toComplete) {
					completions = append(completions, cobra.CompletionWithDesc(sub.Name(), sub.Short))
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			cmd, err := topic(root, args)
			if err != nil {
				return err
			}
			cmd.InitDefaultHelpFlag()
			cmd.InitDefaultVersionFlag()
			cmd.HelpFunc()(cmd, nil)
			return *helped
		},
	}
	return help
}
