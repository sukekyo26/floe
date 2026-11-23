package cli

import (
	"maps"
	"time"

	"floe/internal/link"
)

// ShellWords is shellWords, for the tests outside the package to split a doc's command line as the guide's tests do.
var ShellWords = shellWords

// HomeVars, DefaultPlace and PlaceVar are homeVars, defaultPlace and placeVar, for the tests outside the package: the
// environment of a home, and the default places in it, as each system has them.
var (
	HomeKey      = link.HomeKey
	HomeVars     = homeVars
	DefaultPlace = defaultPlace
	PlaceVar     = placeVar
)

// HomeEnv is the environment of homeVars(home) with vars over it.
func HomeEnv(home string, vars map[string]string) map[string]string {
	env := homeVars(home)
	maps.Copy(env, vars)
	return env
}

// SetDrawLimit sets the memory the commands that draw may hold, for a test that runs floe as a process of its own.
func SetDrawLimit(n uint64) {
	drawLimit = n
}

// SetDrawTime sets how long the server lets one page be drawn, for a test that runs floe as a process of its own.
func SetDrawTime(d time.Duration) {
	drawTime = d
}
