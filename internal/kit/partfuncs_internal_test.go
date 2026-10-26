package kit

import (
	"fmt"
	"strings"
	"testing"
)

// A diff's lines take their kind from their first characters: --- and +++ name the files only outside
// a hunk, and a line of another kind ends the hunk, as the next file's diff --git does.
func TestDiffOf(t *testing.T) {
	t.Parallel()

	d := diffOf("diff --git a/x b/x\nindex 1..2\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n keep\n--- gone\n+++ new\n\n" +
		`\ No newline at end of file` + "\ndiff --git a/y b/y\n--- a/y\n+++ b/y\n@@ -1 +1 @@\n-a\n+b")
	kinds := make([]string, 0, len(d.Lines))
	for _, l := range d.Lines {
		kinds = append(kinds, l.Kind)
	}
	want := "meta meta meta meta hunk  del add  meta meta meta meta hunk del add"
	if got := strings.Join(kinds, " "); got != want || d.Added != 2 || d.Removed != 2 {
		t.Errorf("diffOf = %q, +%d -%d; want %q, +2 -2", got, d.Added, d.Removed, want)
	}
	if d.Lines[5].Text != " keep\n" || d.Lines[len(d.Lines)-1].Text != "+b" {
		t.Errorf("the lines keep their text and newline: %q", d.Lines)
	}
	// Lines of a diff without its header are added and removed all the same.
	if d := diffOf("-old\n+new\n same\n"); d.Added != 1 || d.Removed != 1 || d.Lines[2].Kind != "" {
		t.Errorf("diffOf of bare lines = %+v", d)
	}
}

// A tree nests its paths under the folders they name, those not listed too, in the order they come; a
// path listed again gives the one drawn its note and change.
func TestTreeOf(t *testing.T) {
	t.Parallel()

	item := func(path, note, change string) any {
		return map[string]any{"path": path, "note": note, "change": change}
	}
	nodes := treeOf([]any{
		item("a/b.go", "B", ""),
		item("c.md", "", "added"),
		item("a/d/e.go", "", "removed"),
		item("a/", "A", "changed"),
		item("/a//f/", "", ""),
	})
	var b strings.Builder
	var write func(ns []*treeNode, depth int)
	write = func(ns []*treeNode, depth int) {
		for _, n := range ns {
			fmt.Fprintf(&b, "%s%s %s %s %v\n", strings.Repeat(".", depth), n.Name, n.Note, n.Change, n.Dir)
			write(n.Children, depth+1)
		}
	}
	write(nodes, 0)
	want := "a/ A changed true\n.b.go B  false\n.d/   true\n..e.go  removed false\n.f/   true\nc.md  added false\n"
	if b.String() != want {
		t.Errorf("treeOf =\n%s\nwant\n%s", b.String(), want)
	}
}
