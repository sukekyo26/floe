package tmpl_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// The headings of a report get ids wherever they are written: page.html, Markdown and parts, the second of
// the same text across items numbered.
func TestReportHeadingIDs(t *testing.T) {
	t.Parallel()

	page := render(t, `template: report
lang: ja
sections:
  - heading: 1. 何が起きるか
    parts:
      - text: "### 何が起きるか"
      - cards:
          - title: 何が起きるか
  - heading: 次
    parts:
      - text: "### 何が起きるか\n\n[戻る](#1-何が起きるか)"
`)
	for _, want := range []string{
		`<h1 id="a-title-more">`, `<h2 id="1-何が起きるか">`, `<h3 id="何が起きるか">`, `<h3 id="何が起きるか-1">`,
		`<h2 id="次">`, `<h3 id="何が起きるか-2">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page holds no %s", want)
		}
	}
}

// A link to no part of the page is refused at each item that writes it, written as it is, percent-encoded
// or with character references. A link to a heading, # and #top pass, and so does a link to an id written
// after it; a text fragment (:~: and after it) is left out. An href's character references are read once,
// as a browser reads them.
func TestLinkToNoPart(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<main><h2>まとめ</h2>{{.Data.risks}}{{with .Data.aside}}{{.}}{{end}}</main>`)
	_, err := draw(l, `template: memo
lang: en
risks: |
  <a href="#まとめ">ok</a> <a href="#">top</a> <a href="#TOP">top</a> <a href="#later">later</a>
  <a href="#無い">x</a> <a href="#%E7%84%A1%E3%81%84">x</a> <a href="#a&amp;b">x</a> <a href="#まとめ一覧">x</a>
  <p id="later"></p> <a href="#later:~:text=x">x</a> <a href="#:~:text=x">x</a> <a href="#nope:~:text=x">x</a>
  <p id="a&amp;c"></p> <a href="#a&amp;amp;c">x</a>
aside: <a href="#a&b"></a>
`)
	want := `data.yaml:3:8: risks: links to #無い, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has
data.yaml:3:8: risks: links to #a&b, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has
data.yaml:3:8: risks: links to #まとめ一覧, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has
data.yaml:3:8: risks: links to #nope, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has
data.yaml:3:8: risks: links to #a&amp;c, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has
data.yaml:8:8: aside: links to #a&b, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has`
	if err == nil || err.Error() != want {
		t.Errorf("draw = %v, want\n%s", err, want)
	}
}

// A link page.html writes, which no item does, is told at the template's page.html, and one it draws of
// the data at the data file; none is told while the data's HTML changes the page, whose ids and links are
// not as drawn.
func TestLinkOfPage(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<main><a href="#nowhere">to</a><a href="#{{.Data.aside}}">to</a>{{.Data.risks}}<p>after</p></main>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: <p>x</p>\naside: drawn\n")
	if err == nil || !strings.Contains(err.Error(), filepath.Join("memo", "page.html")+": links to #nowhere,") ||
		!strings.Contains(err.Error(), "\ndata.yaml: links to #drawn,") {
		t.Errorf("draw = %v, want #nowhere at page.html and #drawn at data.yaml", err)
	}
	_, err = draw(l, "template: memo\nlang: en\nrisks: <b>x\naside: nowhere\n")
	if err == nil || strings.Contains(err.Error(), "links to") {
		t.Errorf("draw = %v, want the data's HTML alone", err)
	}
}

// A link the search of the items misses, as Markdown wrote it escaped, is told at the data file.
func TestLinkNotFound(t *testing.T) {
	t.Parallel()

	_, err := draw(builtins, "template: report\nlang: en\nsections:\n  - parts:\n      - text: '[x](#a\\)b)'\n")
	want := `data.yaml: links to #a)b, which is no id of the page: a heading's id is its text in lower case, ` +
		`without punctuation and symbols, the spaces between words made one -, and -1, -2 and on for the second of the same text, or for an id the page already has`
	if err == nil || err.Error() != want {
		t.Errorf("draw = %v, want\n%s", err, want)
	}
}

// A link html/template wrote #ZgotmplZ for, its URL not safe, is told so, not as a link to no heading.
func TestUnsafeLink(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<main><a href="{{.Data.aside}}">to</a>{{.Data.risks}}</main>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: <p>x</p>\naside: javascript:alert(1)\n")
	want := "data.yaml: a link's URL was not safe, so html/template wrote #ZgotmplZ in its place: " +
		"write a URL of http, https or mailto, or a path"
	if err == nil || err.Error() != want {
		t.Errorf("draw = %v, want\n%s", err, want)
	}
}
