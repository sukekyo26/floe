package kit

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	cjkfriendly "github.com/tats-u/goldmark-cjk-friendly/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// markdown draws raw HTML as the text written and puts each table in the table part's box; markdownLine draws one
// line without a paragraph, leaving "-" and "1." as text, and raw HTML as written too.
func TestMarkdown(t *testing.T) {
	t.Parallel()

	html, err := markdownHTML("a <b>x</b>\n\n| h |\n| - |\n| c |\n")
	want := "<p>a &lt;b&gt;x&lt;/b&gt;</p>\n" +
		"<div data-floe-part=\"table\"><table>\n<thead>\n<tr>\n<th>h</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>c</td>\n</tr>\n</tbody>\n</table></div>"
	if err != nil || string(html) != want {
		t.Errorf("markdown = %q, %v; want %q", html, err, want)
	}
	// An HTML block is a paragraph of its lines as written, its last line break dropped (a CR too): one of a tag alone
	// or <div> runs to a blank line, one of <style> to its closing tag or the end; in a tight list's item too.
	for src, want := range map[string]template.HTML{
		"<div>x</div>\n":                     "<p>&lt;div&gt;x&lt;/div&gt;</p>",
		"<prefix>\n**b**\n\n**c**\n":         "<p>&lt;prefix&gt;\n**b**</p>\n<p><strong>c</strong></p>",
		"<style>\np {}\n</style>\n\n**c**\n": "<p>&lt;style&gt;\np {}\n&lt;/style&gt;</p>\n<p><strong>c</strong></p>",
		"<style>\np {}\n\n**c**\n":           "<p>&lt;style&gt;\np {}\n\n**c**</p>",
		"- a\n- <div>b</div>\n":              "<ul>\n<li>a</li>\n<li>\n<p>&lt;div&gt;b&lt;/div&gt;</p>\n</li>\n</ul>",
		"<div>\r\nx\r\n":                     "<p>&lt;div&gt;\r\nx</p>",
		"<script>alert(1)</script>\n":        "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>",
		"a <img src=x onerror=alert(1)> b\n": "<p>a &lt;img src=x onerror=alert(1)&gt; b</p>",
	} {
		if got, err := markdownHTML(src); err != nil || got != want {
			t.Errorf("markdown(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	// * beside CJK letters pairs across a mark (markdown-cjk-friendly), a * meant as a mark there too (LIMIT); _ and
	// Markdown of no CJK letter read as CommonMark.
	for src, want := range map[string]template.HTML{
		"**「ページ一覧」**に変える\n":  "<p><strong>「ページ一覧」</strong>に変える</p>",
		"先に言う。**ロゴ。**画像は後\n": "<p>先に言う。<strong>ロゴ。</strong>画像は後</p>",
		"あ**()**あ\n":                "<p>あ<strong>()</strong>あ</p>",
		"*「斜め」*です\n":                "<p><em>「斜め」</em>です</p>",
		"a**(b)**c\n":               "<p>a**(b)**c</p>",
		"__「題」__に\n":                "<p>__「題」__に</p>",
		"設定は*.yamlと*.jsonを読む\n":     "<p>設定は<em>.yamlと</em>.jsonを読む</p>",
		"設定は\\*.yamlと\\*.jsonを読む\n": "<p>設定は*.yamlと*.jsonを読む</p>",
	} {
		if got, err := markdownHTML(src); err != nil || got != want {
			t.Errorf("markdown(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	line, ok := funcs["markdownLine"].(func(string) (template.HTML, error))
	if !ok {
		t.Fatal("markdownLine is not a func(string) (template.HTML, error)")
	}
	for src, want := range map[string]template.HTML{
		"-": "-", "1. x": "1. x", "# x": "# x", "**a**\n  b": "<strong>a</strong>\nb",
		"/logs/<prefix>": "/logs/&lt;prefix&gt;", "a <script>alert(1)</script>": "a &lt;script&gt;alert(1)&lt;/script&gt;",
		"タブを**「ページ一覧」**に変える。": "タブを<strong>「ページ一覧」</strong>に変える。",
	} {
		if got, err := line(src); err != nil || got != want {
			t.Errorf("markdownLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

// markdownLabel draws a line as markdownLine does, a link written or found as its words, and raw HTML as written.
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
		"Vec<u8> parsing":                        "Vec&lt;u8&gt; parsing",
		"<T>":                                    "&lt;T&gt;",
		"見出しの**「題」**の後":                          "見出しの<strong>「題」</strong>の後",
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
		"~~「消す」~~に":                        "<del>「消す」</del>に",
		"~「消さない」~に":                        "~「消さない」~に",
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

// markdown is goldmark's GFM but its strikethrough and markdown-cjk-friendly: Markdown of English letters alone, with no
// tilde, draws the same, so that an extension goldmark adds to extension.GFM is missed here no longer. The samples hold
// no CJK letter, which markdown-cjk-friendly reads otherwise.
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

// MarkdownFlow draws Markdown in a text's box and Markdown in the box of a template's own Markdown, raw HTML alone too
// (once or twice), and empty Markdown as it is, with no box; part.html's markdown is MarkdownFlow, page.html's Markdown.
func TestMarkdownFlow(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		"":                               "",
		"<div>x</div>\n":                 "<div MARK><p>&lt;div&gt;x&lt;/div&gt;</p></div>",
		"<div>x</div>\n\n<div>y</div>\n": "<div MARK><p>&lt;div&gt;x&lt;/div&gt;</p>\n<p>&lt;div&gt;y&lt;/div&gt;</p></div>",
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

// goldmark-cjk-friendly reads *, _ and ~~ beside letters and marks of no CJK as goldmark does: it copies goldmark's
// ScanDelimiter, so that a change of goldmark's parser/delimiter.go the copy misses fails here.
func TestCJKFriendlyKeepsCommonMark(t *testing.T) {
	t.Parallel()

	plain := goldmark.New(goldmark.WithExtensions(extension.Strikethrough))
	cjk := goldmark.New(goldmark.WithExtensions(cjkfriendly.CJKFriendlyEmphasisAndStrikethrough))
	// Letters, marks, spaces and none, of no CJK: what decides whether a mark opens or closes.
	beside := []string{"", "a", " ", "(", ")", ".", "!", "é", "—", "“", "”", "«", " ", "á", "😀", "→"}
	for _, mark := range []string{"*", "**", "_", "__", "~~"} {
		for _, a := range beside {
			for _, b := range beside {
				for _, c := range beside {
					for _, d := range beside {
						src := []byte(a + mark + b + "x" + c + mark + d)
						var want, got bytes.Buffer
						if err := plain.Convert(src, &want); err != nil {
							t.Fatal(err)
						}
						if err := cjk.Convert(src, &got); err != nil {
							t.Fatal(err)
						}
						if got.String() != want.String() {
							t.Errorf("%q: goldmark-cjk-friendly draws %q, goldmark %q", src, got.String(), want.String())
						}
					}
				}
			}
		}
	}
}
