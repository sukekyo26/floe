package page_test

import (
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/page"
)

func TestTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, html, want string
	}{
		{"title", "<title>Demo  Page</title><p>hi</p>", "Demo Page"},
		{"first h1 without a title", "<h1>One</h1><h1>Two</h1>", "One"},
		{"title wins over h1", "<h1>Head</h1><title>Doc</title>", "Doc"},
		{"plain text of the h1", "  <h1>A &amp; <em>B</em></h1>", "A & B"},
		{"entities", "<title>&lt;x&gt; &quot;q&quot; &#39;s&#39; &#43;&nbsp;</title>", "<x> \"q\" 's' +\u00a0"},
		{"attributes", `<h1 class="big">Big</h1>`, "Big"},
		// As document.title: ASCII whitespace alone is collapsed and trimmed.
		{"unicode spaces", "<title>A&nbsp;B\u3000\u3000C\u2003</title>", "A\u00a0B\u3000\u3000C\u2003"},
		{"ascii spaces", "<title> a\tb\n c\f</title>", "a b c"},
		{"case and newlines", "<TITLE>\n  Upper\n</TITLE>", "Upper"},
		{"svg title is not the page title", `<h1>Sales</h1><svg viewBox="0 0 1 1"><title>Bar</title></svg>`, "Sales"},
		{"element name boundary", "<titlex>no</titlex><h1x>no</h1x><h1>yes</h1>", "yes"},
		// As an HTML parser reads the page: no title in a comment, a script or a textarea, nor an element's
		// tags inside <title>, which holds text alone.
		{"title in a comment", "<!-- <title>old</title> --><title>new</title>", "new"},
		{"title in a script", `<script>"<title>in script</title>"</script><title>page</title>`, "page"},
		{"title in a textarea", "<textarea><title>in</title></textarea><title>page</title>", "page"},
		{"title in nested svgs", "<svg><svg><title>inner</title></svg><title>outer</title></svg><title>page</title>", "page"},
		{"tags in a title are text", "<title>a <b>b</b></title>", "a <b>b</b>"},
		{"> in an attribute", `<h1 title="<x>">Head</h1>`, "Head"},
		{"none", "<p>untitled</p>", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := page.Title(tt.html); got != tt.want {
				t.Errorf("Title(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}

// An element no input may write is refused by its name in whatever namespace the parser puts it; a <meta> of a
// charset alone or of a viewport's name and content is not.
func TestRefused(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ html, tag, want string }{
		{"<style>p{}</style>", "style", "style"},
		{"<svg><style>rect{}</style></svg>", "style", "style"},
		{"<math><style>x</style></math>", "style", "style"},
		{"<svg><BASE/></svg>", "base", "base"},
		{`<base href="https://x/">`, "base", "base"},
		{"<noscript><p>x</p></noscript>", "noscript", "noscript"},
		{"<svg><g><NoScript/></g></svg>", "noscript", "noscript"},
		{`<meta charset="utf-8">`, "meta", ""},
		{`<meta name="VIEWPORT" content="width=device-width">`, "meta", ""},
		{`<meta charset="utf-8" http-equiv="refresh">`, "meta", "meta"},
		{`<meta name="viewport" content="x" http-equiv="refresh">`, "meta", "meta"},
		{`<meta name="generator" content="x">`, "meta", "meta"},
		{"<svg><script>x</script></svg>", "script", ""},
		{`<p class="noscript">style</p>`, "p", ""},
	} {
		doc, err := html.Parse(strings.NewReader(c.html))
		if err != nil {
			t.Fatal(err)
		}
		var n *html.Node
		for d := range doc.Descendants() {
			if d.Type == html.ElementNode && d.Data == c.tag {
				n = d
				break
			}
		}
		if n == nil {
			t.Fatalf("%s: no <%s>", c.html, c.tag)
		}
		if got := page.Refused(n); got != c.want {
			t.Errorf("%s: Refused = %q, want %q", c.html, got, c.want)
		}
	}
}

// A script of floe's drops the lines that are a comment alone, and its argument cannot end it.
func TestScript(t *testing.T) {
	t.Parallel()

	js := "// a note\nfunction (w) {\n  // why\n  return w; // kept\n}\n"
	got, err := page.Script(js, map[string]string{"t": "</script><!--"})
	want := `<script>(function (w) {` + "\n" + `  return w; // kept` + "\n" + `})({"t":"\u003c/script\u003e\u003c!--"});</script>`
	if err != nil || got != want {
		t.Errorf("Script = %s, %v\nwant %s", got, err, want)
	}
}
