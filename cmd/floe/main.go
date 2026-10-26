// Command floe serves agent-written HTML pages on a local server.
package main

import (
	"os"

	"github.com/sukekyo26/floe/internal/cli"
)

func main() {
	// A working directory that cannot be read is "": a relative path is then refused, not read elsewhere.
	dir, _ := os.Getwd() //nolint:errcheck // see above
	os.Exit(cli.Run(os.Args[1:], cli.Env{Stdout: os.Stdout, Stderr: os.Stderr, Dir: dir}))
}
