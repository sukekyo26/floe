package kit

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/page"
)

// The page template new writes and each of floe's templates' holds a viewport in its head, read as
// page.shown reads one: without it, a phone draws a page opened by itself 980px wide and shrinks it.
func TestViewport(t *testing.T) {
	t.Parallel()

	pages := map[string]string{"skeletonPage": skeletonPage}
	found, err := fs.Glob(floeFiles, "floe/templates/*/page.html")
	if err != nil || len(found) == 0 {
		t.Fatalf("floe's templates' pages = %v, %v", found, err)
	}
	for _, name := range found {
		src, err := floeFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		pages[name] = string(src)
	}
	for name, src := range pages {
		tree, err := html.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		head := page.First(tree, atom.Head)
		if head == nil {
			t.Errorf("%s has no head", name)
			continue
		}
		if !slices.ContainsFunc(slices.Collect(head.Descendants()), viewport) {
			t.Errorf("%s has no <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"> in its head", name)
		}
	}
}

func viewport(n *html.Node) bool {
	if n.Type != html.ElementNode || n.Namespace != "" || n.DataAtom != atom.Meta {
		return false
	}
	var name, content bool
	for _, a := range n.Attr {
		if a.Namespace != "" {
			return false
		}
		switch a.Key {
		case "name":
			name = strings.EqualFold(a.Val, "viewport")
		case "content":
			content = a.Val == "width=device-width, initial-scale=1"
		default:
			return false
		}
	}
	return name && content
}
