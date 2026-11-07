package browsertest_test

import "testing"

// TestLookRuleContrast measures every page of floePages as measureContrast and measureRings do: the parts, the templates,
// the themes and their palettes together draw nothing that cannot be read, nor a focus that cannot be seen.
func TestLookRuleContrast(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floePages(t, func(what string) { s.b.measureColours(t, what) })
}
