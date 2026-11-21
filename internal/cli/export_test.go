package cli

import (
	"maps"

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
