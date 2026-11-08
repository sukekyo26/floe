package kit

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// markdown leaves raw HTML out and puts each table in the table part's box; markdownLine draws one line
// without a paragraph, leaving "-" and "1." as text.
func TestMarkdown(t *testing.T) {
	t.Parallel()

	html, err := markdownHTML("a <b>x</b>\n\n| h |\n| - |\n| c |\n")
	want := "<p>a <!-- raw HTML omitted -->x<!-- raw HTML omitted --></p>\n" +
		"<div data-floe-part=\"table\"><table>\n<thead>\n<tr>\n<th>h</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>c</td>\n</tr>\n</tbody>\n</table></div>"
	if err != nil || string(html) != want {
		t.Errorf("markdown = %q, %v; want %q", html, err, want)
	}
	// Raw HTML alone leaves a comment alone, no space at its end, which :empty takes.
	if html, err := markdownHTML("<div>x</div>\n"); err != nil || html != "<!-- raw HTML omitted -->" {
		t.Errorf("markdown of raw HTML alone = %q, %v", html, err)
	}
	line, ok := funcs["markdownLine"].(func(string) (template.HTML, error))
	if !ok {
		t.Fatal("markdownLine is not a func(string) (template.HTML, error)")
	}
	for src, want := range map[string]template.HTML{"-": "-", "1. x": "1. x", "# x": "# x", "**a**\n  b": "<strong>a</strong>\nb"} {
		if got, err := line(src); err != nil || got != want {
			t.Errorf("markdownLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

// markdownLabel draws a line as markdownLine does, a link written or found as its words, and raw HTML as nothing.
func TestMarkdownLabel(t *testing.T) {
	t.Parallel()

	label, ok := funcs["markdownLabel"].(func(string) (template.HTML, error))
	if !ok {
		t.Fatal("markdownLabel is not a func(string) (template.HTML, error)")
	}
	for src, want := range map[string]template.HTML{
		"See [the guide](https://example.com/g)": "See the guide",
		"See https://x.com":                      "See https://x.com",
		"<https://x>":                            "https://x",
		"a@example.com":                          "a@example.com",
		"**bold** and `code`":                    "<strong>bold</strong> and <code>code</code>",
		"Vec<u8> parsing":                        "Vec parsing",
		"<T>":                                    "",
	} {
		if got, err := label(src); err != nil || got != want {
			t.Errorf("markdownLabel(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

// Two tildes strike through and one stays text, in each of floe's Markdown: a range or a path is not struck unasked.
func TestStrikethrough(t *testing.T) {
	t.Parallel()

	line, okLine := funcs["markdownLine"].(func(string) (template.HTML, error))
	label, okLabel := funcs["markdownLabel"].(func(string) (template.HTML, error))
	if !okLine || !okLabel {
		t.Fatal("markdownLine or markdownLabel is not a func(string) (template.HTML, error)")
	}
	for src, want := range map[string]template.HTML{
		"2026-11-01~11-08 · 見積 2~3 日":      "2026-11-01~11-08 · 見積 2~3 日",
		"コメント「~/work・Docker・~/.gitconfig」": "コメント「~/work・Docker・~/.gitconfig」",
		"~~消す~~ と ~消さない~":                  "<del>消す</del> と ~消さない~",
		"~~a~ b~":                          "~~a~ b~",
		"~a~~":                             "~a~~",
		"x ~~~three~~~":                    "x ~~~three~~~",
		"~~a~~~~b~~":                       "<del>a~~~~b</del>",
		"~~~a~~":                           "~~~a~~",
		"~~a~~~":                           "~~a~~~",
		"x~~a~~y":                          "x<del>a</del>y",
		`\~~a~~`:                           "~~a~~",
		"`~~code~~`":                       "<code>~~code~~</code>",
		"~~**太字**~~":                       "<del><strong>太字</strong></del>",
		"[~~link~~](http://x)":             `<a href="http://x"><del>link</del></a>`,
		"~https://example.com~":            `~<a href="https://example.com">https://example.com</a>~`,
		"~~https://example.com~~":          `<del><a href="https://example.com">https://example.com</a></del>`,
	} {
		if got, err := line(src); err != nil || got != want {
			t.Errorf("markdownLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	for src, want := range map[string]template.HTML{"~x~ and ~~y~~": "~x~ and <del>y</del>"} {
		if got, err := label(src); err != nil || got != want {
			t.Errorf("markdownLabel(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	for src, want := range map[string]template.HTML{
		"~x~ and ~~y~~": "<p>~x~ and <del>y</del></p>",
		"| a | b |\n|---|---|\n| ~x~ | ~~y~~ |\n": `<div data-floe-part="table"><table>` + "\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n" +
			"<tbody>\n<tr>\n<td>~x~</td>\n<td><del>y</del></td>\n</tr>\n</tbody>\n</table></div>",
		"- [x] ~~done~~ ~not~\n": "<ul>\n<li><input checked=\"\" disabled=\"\" type=\"checkbox\"> <del>done</del> ~not~</li>\n</ul>",
		"~~~three~~~\n":          `<pre><code class="language-three~~~"></code></pre>`,
	} {
		if got, err := markdownHTML(src); err != nil || got != want {
			t.Errorf("markdown(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

// markdown is goldmark's GFM but its strikethrough: Markdown with no tilde draws the same, so that an extension goldmark
// adds to extension.GFM is missed here no longer.
func TestMarkdownIsGFM(t *testing.T) {
	t.Parallel()

	gfm := goldmark.New(goldmark.WithExtensions(extension.GFM))
	for _, src := range []string{
		"| a | b |\n|:--|--:|\n| 1 | **2** |\n",
		"- [ ] todo\n- [x] done\n",
		"See https://example.com/a?b=1, www.example.com and a@example.com.\n",
		"A note[^1].\n\n[^1]: The note.\n",
		"# Head\n\n> quote\n\n1. one\n2. two\n\n```go\nx := 1\n```\n",
		"*em* **strong** `code` [link](https://example.com) ![img](data:image/png;base64,AA==)\n",
	} {
		var want, got bytes.Buffer
		if err := gfm.Convert([]byte(src), &want); err != nil {
			t.Fatal(err)
		}
		if err := markdown.Convert([]byte(src), &got); err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Errorf("markdown(%q) = %q; GFM draws %q", src, got.String(), want.String())
		}
	}
}

// MarkdownFlow draws Markdown in a text's box and Markdown in the box of a template's own Markdown, and what draws no
// block (empty, raw HTML alone, once or twice) as it is, with no box; part.html's markdown is MarkdownFlow, page.html's
// Markdown.
func TestMarkdownFlow(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		"":                               "",
		"<div>x</div>\n":                 "<!-- raw HTML omitted -->",
		"<div>x</div>\n\n<div>y</div>\n": "<!-- raw HTML omitted -->\n<!-- raw HTML omitted -->",
		"a\n\nb":                         "<div MARK><p>a</p>\n<p>b</p></div>",
	} {
		for name, draw := range map[string]func(string) (template.HTML, error){"data-floe-text": MarkdownFlow, "data-floe-markdown": Markdown} {
			if got, err := draw(src); err != nil || string(got) != strings.ReplaceAll(want, "MARK", name) {
				t.Errorf("drawn in %s, %q = %q, %v; want %q", name, src, got, err, strings.ReplaceAll(want, "MARK", name))
			}
		}
	}
	for name, fns := range map[string]template.FuncMap{"part.html": partFuncsAll(), "page.html": pageFuncs()} {
		md, ok := fns["markdown"].(func(string) (template.HTML, error))
		if !ok {
			t.Fatalf("%s's markdown is not a func(string) (template.HTML, error)", name)
		}
		got, err := md("a")
		if want := map[string]template.HTML{"part.html": "<div data-floe-text><p>a</p></div>", "page.html": "<div data-floe-markdown><p>a</p></div>"}[name]; err != nil || got != want {
			t.Errorf("%s's markdown = %q, %v; want %q", name, got, err, want)
		}
	}
}

// Fenced finds the line of the first fenced code block as goldmark reads it, in a list or a quote and unclosed too,
// and no code in a line, in a table or indented.
func TestFenced(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]int{
		"Run this:\n\n```sh\nls\n```\n":                3,
		"- item\n\n  ~~~\n  nested in a list\n  ~~~\n": 3,
		"> quote\n>\n> ```\n> q\n> ```\n":              3,
		"```\n```\n\n```go\nx\n```":                    1,
		"```\nunclosed fence":                          1,
		"```x``` is inline code, not a fence":          0,
		"Para.\n\n    ```\n    indented four\n":        0,
		"- item\n\n      indented in list\n":           0,
		"| a |\n|---|\n| `x` |\n":                      0,
	} {
		if got := Fenced(src); got != want {
			t.Errorf("Fenced(%q) = %d, want %d", src, got, want)
		}
	}
}
