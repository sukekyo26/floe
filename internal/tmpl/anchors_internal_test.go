package tmpl

import "testing"

func TestHeadingID(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		"1. テンプレートの固定の言葉を訳せない": "1-テンプレートの固定の言葉を訳せない",
		"「まとめ」・一覧（2026 年）":     "まとめ一覧2026-年",
		"floe check を回す":       "floe-check-を回す",
		" Go  Notes_v2 ":       "go-notes_v2",
		"\n  Foo\n\tBar\n":     "foo-bar",
		"!!!":                  "",
	} {
		if got := headingID(text); got != want {
			t.Errorf("headingID(%q) = %q, want %q", text, got, want)
		}
	}
}

// Ids are free of every id and <a> name of the page, those after the heading too, and of the ids given
// before; a heading ends where the parser ends it, its text without a part's; each heading gets its own
// id where the parser moves it out of a table; the rest of the page is left byte for byte, a tag left open
// at the end too.
func TestWithHeadingIDs(t *testing.T) {
	t.Parallel()

	for page, want := range map[string]string{
		`<H2 class=x>A <code>b</code></H2><div id="a-b">`:       `<H2 id="a-b-1" class=x>A <code>b</code></H2><div id="a-b">`,
		`<h2>a</h2><h3>a</h3><h4>a-1</h4>`:                      `<h2 id="a">a</h2><h3 id="a-1">a</h3><h4 id="a-1-1">a-1</h4>`,
		`<h2 id=own>a</h2><h2>own</h2><h2>!</h2>`:               `<h2 id=own>a</h2><h2 id="own-1">own</h2><h2>!</h2>`,
		`<script>"<h2>x</h2>"</script><p>no heading`:            `<script>"<h2>x</h2>"</script><p>no heading`,
		`<h2>a & "b"</h2><h2>open`:                              `<h2 id="a--b">a & "b"</h2><h2 id="open">open`,
		`<h2>a</h3><p>body</p><h2>b</h2>`:                       `<h2 id="a">a</h3><p>body</p><h2 id="b">b</h2>`,
		`<h2>c<h3>d</h3></h2>`:                                  `<h2 id="c">c<h3 id="d">d</h3></h2>`,
		`<h2/>e</h2>`:                                           `<h2 id="e"/>e</h2>`,
		`<a name="summary"></a><h2>Summary</h2>`:                `<a name="summary"></a><h2 id="summary-1">Summary</h2>`,
		`<div><h2>Title</div><p>long rest</p><h2>B</h2>`:        `<div><h2 id="title">Title</div><p>long rest</p><h2 id="b">B</h2>`,
		`<ul><li><h3>X</li><li>more text</li></ul>`:             `<ul><li><h3 id="x">X</li><li>more text</li></ul>`,
		`<h2>a <code>b</code> <span><em>c</em></span></h2>`:     `<h2 id="a-b-c">a <code>b</code> <span><em>c</em></span></h2>`,
		`<div><h3>Q<div>in</div>R</h3></div>`:                   `<div><h3 id="qinr">Q<div>in</div>R</h3></div>`,
		`<table><tr><td><h2>A</h2></td></tr><h2>B</h2></table>`: `<table><tr><td><h2 id="a">A</h2></td></tr><h2 id="b">B</h2></table>`,
		`<h3>A <span data-floe-part="cell">Take</span></h3>`:    `<h3 id="a">A <span data-floe-part="cell">Take</span></h3>`,
		`<h2>a<script>x</script></h2><p>x<div`:                  `<h2 id="a">a<script>x</script></h2><p>x<div`,
	} {
		if got, _ := withHeadingIDs([]byte(page)); string(got) != want {
			t.Errorf("withHeadingIDs(%q) =\n%s\nwant\n%s", page, string(got), want)
		}
	}
}
