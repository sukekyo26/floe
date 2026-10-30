package kit

import (
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/page"
)

// The page template new writes and each of floe's templates' holds a viewport in its head: without it, a phone
// draws a page opened by itself 980px wide and shrinks it. Which <meta> a page may have is page's to tell.
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
	for _, name := range slices.Sorted(maps.Keys(pages)) {
		tree, err := html.Parse(strings.NewReader(pages[name]))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(slices.Collect(page.First(tree, atom.Head).Descendants()), viewport) {
			t.Errorf("%s has no <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"> in its head", name)
		}
	}
}

func viewport(n *html.Node) bool {
	return n.DataAtom == atom.Meta &&
		slices.Contains(n.Attr, html.Attribute{Key: "name", Val: "viewport"}) &&
		slices.Contains(n.Attr, html.Attribute{Key: "content", Val: "width=device-width, initial-scale=1"})
}

// The docs show the page template new writes as it is.
func TestSkeletonInDocs(t *testing.T) {
	t.Parallel()

	for _, doc := range []string{"../../docs/templates.md", "../../docs/templates.ja.md"} {
		src, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "```html\n"+skeletonPage+"```\n") {
			t.Errorf("%s does not show skeletonPage as it is", doc)
		}
	}
}
