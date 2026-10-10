// Package cli builds and runs the floe command tree.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/version"
)

// Run executes floe with args and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	root := newRoot()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		// SilenceErrors leaves printing to this single boundary.
		_, _ = fmt.Fprintf(stderr, "floe: %v\n", err)
		return 1
	}
	return 0
}

func newRoot() *cobra.Command {
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
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the floe version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "floe %s\n", version.Version)
			return err //nolint:wrapcheck // a stdout write error needs no extra context
		},
	})
	return root
}
