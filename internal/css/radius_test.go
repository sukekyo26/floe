package css_test

import (
	"slices"
	"testing"

	"floe/internal/css"
)

// TestRadiusLength keeps the corners floe reads as one to one item each: a length or a function passes, two items or a
// / are found, and --radius, which floe's parts read as it is, is not read.
func TestRadiusLength(t *testing.T) {
	t.Parallel()

	pass := []string{
		"5px", "0", "var(--hand)", "calc(2px + 1px)", "min(4px, 2vw)", "var(--a, 4px)", "5px !important",
		"/* a */ 5px /* b */",
	}
	for _, v := range pass {
		for _, name := range []string{"--radius-small", "--radius-pill"} {
			if got := css.RadiusLength([]byte(":scope { " + name + ": " + v + "; }")); got != nil {
				t.Errorf("%s: %s: got %v, want none", name, v, got)
			}
		}
	}
	found := []string{"5px 3px", "5px / 3px", "5px/3px", "5px, 3px", "var(--a) var(--b)", "calc(1px) 2px", "5px 3px !important"}
	for _, v := range found {
		got := css.RadiusLength([]byte(":scope {\n  --radius-pill: " + v + ";\n}"))
		if len(got) != 1 || got[0].Line != 2 || got[0].Name != "--radius-pill" {
			t.Errorf("%s: got %v, want --radius-pill on line 2", v, got)
		}
	}
	if got := css.RadiusLength([]byte(":scope { --radius-small: 5px 3px }")); len(got) != 1 || got[0].Value != "5px 3px" {
		t.Errorf("a value up to its block's }: got %v", got)
	}
	notRead := []string{
		":scope { --radius: 5px 3px; }",                     // floe's parts read --radius as it is
		":scope { border-radius: var(--radius-pill) 4px; }", // read, not set
		":scope { --Radius-Pill: 5px 3px; }",                // another variable: CSS tells them apart
		":scope { --radius-pill:hover { color: red } }",     // a nested rule's prelude, not a declaration
	}
	for _, src := range notRead {
		if got := css.RadiusLength([]byte(src)); got != nil {
			t.Errorf("%s: got %v, want none", src, got)
		}
	}
	two := css.RadiusLength([]byte(":scope { --radius-small: 1px 2px; --radius-pill: 3px 4px; }"))
	if names := []string{two[0].Name, two[len(two)-1].Name}; len(two) != 2 || !slices.Equal(names, []string{"--radius-small", "--radius-pill"}) {
		t.Errorf("two in a block: got %v", two)
	}
}
