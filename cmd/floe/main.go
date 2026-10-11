// Command floe serves agent-written HTML pages on a local server.
package main

import (
	"context"
	"os"

	"github.com/sukekyo26/floe/internal/cli"
)

func main() {
	// An unreadable working directory only matters to publish, which resolves files against it.
	dir, _ := os.Getwd() //nolint:errcheck // see above
	os.Exit(cli.Run(context.Background(), os.Args[1:], cli.Env{
		Getenv: os.Getenv, Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr, Environ: os.Environ(),
	}))
}
