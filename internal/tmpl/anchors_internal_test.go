package tmpl

import "testing"

func TestHeadingID(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		"1. テンプレートの固定の言葉を訳せない": "1-テンプレートの固定の言葉を訳せない",
		"「まとめ」・一覧（2026 年）":     "まとめ一覧2026-年",
		"floe check を回す":       "floe-check-を回す",
		" Go  Notes_v2 ":       "go--notes_v2",
		"!!!":                  "",
	} {
		if got := headingID(text); got != want {
			t.Errorf("headingID(%q) = %q, want %q", text, got, want)
		}
	}
}

// Ids are free of every id of the page, those after the heading too, and of the ids given before; the
// rest of the page is left byte for byte.
func TestWithHeadingIDs(t *testing.T) {
	t.Parallel()

	for page, want := range map[string]string{
		`<H2 class=x>A <code>b</code></H2><div id="a-b">`: `<H2 id="a-b-1" class=x>A <code>b</code></H2><div id="a-b">`,
		`<h2>a</h2><h3>a</h3><h4>a-1</h4>`:                `<h2 id="a">a</h2><h3 id="a-1">a</h3><h4 id="a-1-1">a-1</h4>`,
		`<h2 id=own>a</h2><h2>own</h2><h2>!</h2>`:         `<h2 id=own>a</h2><h2 id="own-1">own</h2><h2>!</h2>`,
		`<script>"<h2>x</h2>"</script><p>no heading`:      `<script>"<h2>x</h2>"</script><p>no heading`,
		`<h2>a & "b"</h2><h2>open`:                        `<h2 id="a--b">a & "b"</h2><h2>open`,
	} {
		if got := string(withHeadingIDs([]byte(page))); got != want {
			t.Errorf("withHeadingIDs(%q) =\n%s\nwant\n%s", page, got, want)
		}
	}
}
