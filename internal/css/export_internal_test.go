package css

import (
	"slices"
	"strings"
)

// Variables returns the names of the variables src sets, in order, for the tests of package css_test.
func Variables(src []byte) []string {
	found := declarations(src, func(p string) bool { return strings.HasPrefix(p, "--") })
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
