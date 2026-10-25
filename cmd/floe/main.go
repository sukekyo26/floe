// Command floe serves agent-written HTML pages on a local server.
package main

import (
	"os"

	"github.com/sukekyo26/floe/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Env{Stdout: os.Stdout, Stderr: os.Stderr}))
}
