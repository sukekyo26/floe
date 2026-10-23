package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// Every command floe adds declares the settings it uses, so that a new one is not told the mistakes of
// settings it does not use, nor left untold of those it does.
func TestEveryCommandDeclaresItsSettings(t *testing.T) {
	t.Parallel()

	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if _, ok := cmd.Annotations[declared]; cmd.Runnable() && !ok {
			t.Errorf("%s declares no settings", cmd.CommandPath())
		}
		dirs := 0
		for s := range dirKeys {
			if use(cmd.Annotations[string(s)]) == needs {
				dirs++
			}
		}
		if dirs > 1 {
			t.Errorf("%s needs %d directories; tell the defaults one file holds up in one line (see the LIMIT in settle)",
				cmd.CommandPath(), dirs)
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(newRoot(Env{}))
}
