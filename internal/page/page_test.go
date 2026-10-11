package page_test

import (
	"strings"
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

func TestIsDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		html string
		want bool
	}{
		{"<!DOCTYPE html><p>x</p>", true},
		{"<html lang=ja>", true},
		{"<head>", true},
		{"<body>", true},
		{"<title>t</title>", true},
		{"<p>fragment</p>", false},
		{"<header>not a head</header>", false},
		{`<svg viewBox="0 0 1 1"><title>Chart</title></svg><h1>Frag</h1>`, false},
	}
	for _, tt := range tests {
		if got := page.IsDocument(tt.html); got != tt.want {
			t.Errorf("IsDocument(%q) = %v, want %v", tt.html, got, tt.want)
		}
	}
}

func TestStartsAsDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		head string
		want bool
	}{
		{"<!doctype html><title>Doc</title>", true},
		{`<svg viewBox="0 0 1 1"><title>Chart</title></svg><h1>Frag</h1>`, false},
		{"<body><svg><title>x</title></svg>", true},
		{"<!-- " + strings.Repeat("x", 2000) + " --><!doctype html>", false},
	}
	for _, tt := range tests {
		head := []byte(tt.head)
		if len(head) > page.SniffSize {
			head = head[:page.SniffSize]
		}
		if got := page.StartsAsDocument(head); got != tt.want {
			t.Errorf("StartsAsDocument(%.40q) = %v, want %v", tt.head, got, tt.want)
		}
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run("a document is kept as is", func(t *testing.T) {
		t.Parallel()

		doc := "<!doctype html><title>Doc</title><p>x</p>\n"
		got, err := page.Wrap(doc, "ignored")
		if err != nil || string(got) != doc {
			t.Errorf("Wrap = %q, %v; want the document unchanged", got, err)
		}
	})

	t.Run("a fragment is wrapped verbatim with an escaped title", func(t *testing.T) {
		t.Parallel()

		frag := "  <h1>A &amp; <em>B</em></h1>\n<p>keeps $& and $1 and {{.Content}}</p>\n\n"
		got, err := page.Wrap(frag, `A & B "{{.Content}}"`)
		if err != nil {
			t.Fatal(err)
		}
		s := string(got)
		if !strings.HasPrefix(strings.ToLower(s), "<!doctype html>\n") {
			t.Errorf("wrapped page starts with %.30q, want <!doctype html>", s)
		}
		if !strings.Contains(s, `<meta name="generator" content="floe template">`) {
			t.Error("wrapped page lacks the template's generator mark")
		}
		// The page runs without an HTTP cache (sandbox), so its fonts must not swap in on every load.
		if !strings.Contains(s, "&display=block\">") {
			t.Error("the wrapped page does not load its fonts with display=block")
		}
		if strings.Count(s, frag) != 1 {
			t.Errorf("the fragment is not inserted exactly once and verbatim:\n%s", s)
		}
		if want := "<title>A &amp; B &#34;{{.Content}}&#34;</title>"; !strings.Contains(s, want) {
			t.Errorf("wrapped page lacks %q", want)
		}
		if page.Title(s) != `A & B "{{.Content}}"` {
			t.Errorf("Title(wrapped) = %q", page.Title(s))
		}
	})
}
