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
		{"entities", "<title>&lt;x&gt; &quot;q&quot; &#39;s&#39; &#43;&nbsp;</title>", `<x> "q" 's' +`},
		{"attributes", `<h1 class="big">Big</h1>`, "Big"},
		{"unicode spaces", "<title>A&nbsp;B\u3000\u3000C\u2003</title>", "A B C"},
		{"case and newlines", "<TITLE>\n  Upper\n</TITLE>", "Upper"},
		{"svg title is not the page title", `<h1>Sales</h1><svg viewBox="0 0 1 1"><title>Bar</title></svg>`, "Sales"},
		{"element name boundary", "<titlex>no</titlex><h1x>no</h1x><h1>yes</h1>", "yes"},
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
