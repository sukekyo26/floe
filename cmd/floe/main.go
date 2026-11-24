// Command floe serves agent-written HTML pages on a local server.
package main

import (
	"os"

	"floe/internal/cli"
)

func main() {
	// A working directory that cannot be read is "": a relative path is then refused, not read elsewhere.
	dir, _ := os.Getwd() //nolint:errcheck // see above
	env := cli.Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Dir: dir, Getenv: os.Getenv, Exit: os.Exit}
	os.Exit(cli.Run(os.Args[1:], env))
}
