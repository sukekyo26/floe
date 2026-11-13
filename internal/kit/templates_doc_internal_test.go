package kit

import (
	"maps"
	"slices"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// The table of names in docs/agents/templates.md, which floe template guide prints, has a row for every name a page.html
// can use that floe gives: the page's own (.Title, .Lang, .Data, .Calc), the functions floe adds to html/template's, and
// asset and word, which a template's files give; and no other.
// LIMIT: .Title, .Lang, .Data, .Calc, asset and word are copied by hand from draw.Draw (which kit cannot import) and the
// template's own funcs; a name draw comes to pass is not caught. Test it from draw if the names change.
func TestTemplatesDocNames(t *testing.T) {
	t.Parallel()

	want := append(slices.Collect(maps.Keys(pageFuncs())), ".Title", ".Lang", ".Data", ".Calc", "asset", "word")
	slices.Sort(want)
	got := docFirstColumn(t, "templates.md", "page.html")
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the table of names in docs/agents/templates.md names %v, a page.html can use %v: write a row for each, and no other", got, want)
	}
}

// The first html block of docs/agents/templates.md is the page.html that floe template new writes: the doc says so.
func TestTemplatesDocSkeleton(t *testing.T) {
	t.Parallel()

	for n := range agentDoc(t, "templates.md").Descendants() {
		if n.DataAtom == atom.Code && slices.ContainsFunc(n.Attr, func(a html.Attribute) bool { return a.Key == "class" && a.Val == "language-html" }) {
			if got, want := textOf(n), string(TemplateSkeleton("memo")["page.html"]); got != want {
				t.Errorf("the first html block of docs/agents/templates.md is\n%s\nfloe template new writes\n%s\nwrite the page as it is", got, want)
			}
			return
		}
	}
	t.Error("docs/agents/templates.md has no html block: write the page.html that floe template new writes")
}
