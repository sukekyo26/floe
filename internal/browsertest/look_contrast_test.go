package browsertest_test

import "testing"

// lookContrast measures every page of floePages as measureContrast and measureRings do: the parts, the templates,
// the themes and their palettes together draw nothing that cannot be read, nor a focus that cannot be seen.
func lookContrast(t *testing.T) { //nolint:paralleltest,thelper // a subtest of TestLookRule; the browser tests share one Chrome
	s := newSite(t)
	s.floePages(t, func(what string) { s.b.measureColours(t, what) })
}
