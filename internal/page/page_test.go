package page_test

import (
	"testing"

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
