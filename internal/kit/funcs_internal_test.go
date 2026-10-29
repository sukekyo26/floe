package kit

import (
	"html/template"
	"testing"
)

// markdown leaves raw HTML out and puts each table in the table part's box; markdownLine draws one line
// without a paragraph, leaving "-" and "1." as text.
func TestMarkdown(t *testing.T) {
	t.Parallel()

	html, err := markdownHTML("a <b>x</b>\n\n| h |\n| - |\n| c |\n")
	want := "<p>a <!-- raw HTML omitted -->x<!-- raw HTML omitted --></p>\n" +
		"<div data-floe-part=\"table\"><table>\n<thead>\n<tr>\n<th>h</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>c</td>\n</tr>\n</tbody>\n</table></div>\n"
	if err != nil || string(html) != want {
		t.Errorf("markdown = %q, %v; want %q", html, err, want)
	}
	line, ok := funcs["markdownLine"].(func(string) (template.HTML, error))
	if !ok {
		t.Fatal("markdownLine is not a func(string) (template.HTML, error)")
	}
	for src, want := range map[string]template.HTML{"-": "-", "1. x": "1. x", "**a**\n  b": "<strong>a</strong>\nb"} {
		if got, err := line(src); err != nil || got != want {
			t.Errorf("markdownLine(%q) = %q, %v; want %q", src, got, err, want)
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
