// Command floe serves agent-written HTML pages on a local server.
package main

import (
	"context"
	"os"

	"github.com/sukekyo26/floe/internal/cli"
)

func main() {
	// An unreadable working directory matters only to what is resolved against it: a relative file, or a
	// relative directory of the settings, which is refused.
	dir, _ := os.Getwd() //nolint:errcheck // see above
	os.Exit(cli.Run(context.Background(), os.Args[1:], cli.Env{
		Getenv: os.Getenv, Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr, Environ: os.Environ(),
	}))
}
