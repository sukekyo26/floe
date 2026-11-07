package kit

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// A diff's lines take their kind from their first characters: --- and +++ name the files only outside
// a hunk, which ends when the lines its head counts are used up, or at a line of another kind, as the
// next file's diff --git.
func TestDiffOf(t *testing.T) {
	t.Parallel()

	d := diffOf("diff --git a/x b/x\nindex 1..2\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n keep\n--- gone\n+++ new\n\n" +
		`\ No newline at end of file` + "\ndiff --git a/y b/y\n--- a/y\n+++ b/y\n@@ -1 +1 @@\n-a\n+b")
	want := "meta meta meta meta hunk  del add  meta meta meta meta hunk del add"
	if got := kindsOf(d); got != want || d.Added != 2 || d.Removed != 2 {
		t.Errorf("diffOf = %q, +%d -%d; want %q, +2 -2", got, d.Added, d.Removed, want)
	}
	if d.Lines[5].Text != " keep\n" || d.Lines[len(d.Lines)-1].Text != "+b" {
		t.Errorf("the lines keep their text and newline: %q", d.Lines)
	}
	// Lines of a diff without its header are added and removed all the same.
	if d := diffOf("-old\n+new\n same\n"); d.Added != 1 || d.Removed != 1 || d.Lines[2].Kind != "" {
		t.Errorf("diffOf of bare lines = %+v", d)
	}
	for _, c := range []struct{ text, kinds string }{
		// Joined without diff --git: the head's counts end each hunk.
		{
			"--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old x\n+new x\n--- a/y\n+++ b/y\n@@ -1 +1 @@\n-old y\n+new y\n",
			"meta meta hunk del add meta meta hunk del add",
		},
		{"@@ -1,0 +1,0 @@\n--- x\n", "hunk meta"},
		{"@@ -1,1 +1,2 @@\n-a\n-b\n+c\n+d\n--- e\n", "hunk del del add add meta"},
		{"@@ -1 +1 @@\n--- x\n+y\n", "hunk del add"},
		{"@@ -1 +1,2 @@\n-a\n+b\n+++ c\n", "hunk del add add"},
		{"@@ -1,2 +1,2 @@\n a\n-b\n" + `\ No newline at end of file` + "\n+++ c\n+d\n", "hunk  del meta add add"},
		{"@@ -1,2 +1,2 @@\n\n-a\n+b\n--- c\n", "hunk  del add meta"},
		{"@@ -1,3 +1,3 @@\n-a\ndiff --git a/y b/y\n--- a/y\n+++ b/y\n", "hunk del meta meta meta"},
		{"--- a/q.sql\n+++ b/q.sql\n@@ -1,2 +1,1 @@\n--- note\n select 1\n", "meta meta hunk del "},
		// A head whose counts are not read lasts until a line of another kind, as without them.
		{"@@ hand @@\n-a\n+b\n--- c\n", "hunk del add del"},
		{"@@ -1,99999999999999999999 +1 @@\n-a\n+b\n--- c\n", "hunk del add del"},
	} {
		if got := kindsOf(diffOf(c.text)); got != c.kinds {
			t.Errorf("diffOf(%q) = %q; want %q", c.text, got, c.kinds)
		}
	}
}

// kindsOf returns the kinds of d's lines, joined by spaces.
func kindsOf(d diffText) string {
	kinds := make([]string, 0, len(d.Lines))
	for _, l := range d.Lines {
		kinds = append(kinds, l.Kind)
	}
	return strings.Join(kinds, " ")
}

// A tree nests its paths under the folders they name, those not listed too, in the order they come; a
// path listed again gives the one drawn its note and change.
func TestTreeOf(t *testing.T) {
	t.Parallel()

	item := func(path, note, change string) any {
		return map[string]any{"path": path, "note": note, "change": change}
	}
	nodes := treeOf([]any{
		item("a/b.go", "B", ""),
		item("c.md", "", "added"),
		item("a/d/e.go", "", "removed"),
		item("a/", "A", "changed"),
		item("/a//f/", "", ""),
	})
	var b strings.Builder
	var write func(ns []*treeNode, depth int)
	write = func(ns []*treeNode, depth int) {
		for _, n := range ns {
			fmt.Fprintf(&b, "%s%s %s %s %v\n", strings.Repeat(".", depth), n.Name, n.Note, n.Change, n.Dir)
			write(n.Children, depth+1)
		}
	}
	write(nodes, 0)
	want := "a/ A changed true\n.b.go B  false\n.d/   true\n..e.go  removed false\n.f/   true\nc.md  added false\n"
	if b.String() != want {
		t.Errorf("treeOf =\n%s\nwant\n%s", b.String(), want)
	}
}

// A script whose js an HTML tokenizer would not read whole as its script's text is refused: a </script, however
// written, ends the script there, and a <!-- before a <script moves where it ends; a <!-- alone, CRLF, a key
// __proto__ and a </script> in data are drawn.
func TestScriptOf(t *testing.T) {
	t.Parallel()

	for _, js := range []string{
		`function (el) { el.innerHTML = "</script>" }`, "x = '</SCRIPT '", "x = '</script/'", `f("<!--<script>")`,
	} {
		var e *usererr.Error
		if _, err := scriptOf(js, nil); !errors.As(err, &e) || e.Key != "kit_script_end" {
			t.Errorf("%q: scriptOf = %v, want kit_script_end", js, err)
		}
	}
	for _, js := range []string{"function (el, data) {}", "f(1) <!-- legacy", "function (el) {\r\n  el.x = 1\r\n}"} {
		if _, err := scriptOf(js, map[string]any{"__proto__": 1, "t": "</script><!--<script>"}); err != nil {
			t.Errorf("%q: scriptOf = %v", js, err)
		}
	}
	got, err := scriptOf("function (el, data) {}", map[string]any{"__proto__": "<b>"})
	want := `<div></div><script>{const floeScript = function (el, data) {}` + "\n" +
		`;floeScript(document.currentScript.previousElementSibling, JSON.parse("{\"__proto__\":\"\\u003cb\\u003e\"}"));}</script>`
	if err != nil || string(got) != want {
		t.Errorf("scriptOf = %s, %v\nwant %s", got, err, want)
	}
}

// The copy button's script is in the page's language, English for one without a catalog, the same script for one
// language however often it is drawn, and one script read whole; copyButton not bound to a page is floe's failure.
func TestCopyButton(t *testing.T) {
	t.Parallel()

	draw := func(tag string) string {
		got, err := copyButton(tag)()
		if err != nil {
			t.Fatalf("%s: copyButton = %v", tag, err)
		}
		return string(got)
	}
	ja, en := draw("ja"), draw("en")
	if !strings.Contains(ja, `"copied":"コピーしました","copy":"コピー","failed":"コピーできませんでした"`) ||
		!strings.Contains(en, `"copied":"Copied","copy":"Copy","failed":"Could not copy"`) {
		t.Errorf("copyButton words:\n ja %s\n en %s", ja[len(ja)-120:], en[len(en)-120:])
	}
	if draw("JA_jp") != ja || draw("pt-BR") != en {
		t.Error("copyButton does not fall back as the catalog does: ja-JP to ja, pt-BR to en")
	}
	if body := strings.TrimSuffix(strings.TrimPrefix(ja, "<script>"), "</script>"); !wholeScript(body) || strings.Contains(body, "//") {
		t.Errorf("copyButton is not one script without its comments: %s", ja)
	}
	if _, err := copyButtonOf(); !errors.Is(err, ErrFunc) {
		t.Errorf("copyButtonOf() = %v, want ErrFunc", err)
	}
}

// A bar's width is its share of its of, when written, else of the longest value of the bars without of, and every
// such bar is 0 wide when all their values are 0; a value below 0, which would make a width CSS drops, is refused,
// and -0 is 0 wide; so are an of not above 0 and a value over its of. An of of nil, as Walk sets one the data leaves
// out, is none. The mistakes of every item are told, each with its bar's number and label, and its numbers as num
// writes them; an item not written as keys and
// values is refused, told by its number.
func TestBarsOf(t *testing.T) {
	t.Parallel()

	item := func(v any) any { return map[string]any{"label": "x", "value": v} }
	of := func(label string, v, of any) any { return map[string]any{"label": label, "value": v, "of": of} }
	notItem := "bars takes items written as keys and values (label, value, of, tone); item 2 is not"
	for _, c := range []struct {
		items []any
		want  string
	}{
		{[]any{item(1), item(4), item(2.5)}, "25 100 62.5"},
		{[]any{item(0), item(0)}, "0 0"},
		{[]any{item(math.Copysign(0, -1)), item(4)}, "0 100"},
		{[]any{item(3), item(-2)}, `bars: bar 2 "x"'s value takes a number of 0 or more, not -2`},
		{[]any{item(3), item(-0.5)}, `bars: bar 2 "x"'s value takes a number of 0 or more, not -0.5`},
		{[]any{item(3), item("x")}, `bars: bar 2 "x"'s value takes a number, not "x"`},
		{[]any{item(3), item(nil)}, `bars: bar 2 "x"'s value takes a number; none is written, or it is null`},
		{[]any{item(3), map[string]any{"label": "x"}}, `bars: bar 2 "x"'s value takes a number; none is written, or it is null`},
		{[]any{item(1), 3}, notItem},
		{[]any{item(1), ""}, notItem},
		{[]any{item(1), nil}, notItem},
		{[]any{item(1), []any{1, 2}}, notItem},
		// A bar with of is not a share of the longest, nor counted in it: c would be 1875 wide, b 13.33.
		{[]any{item(2), item(4), of("c", 30, 40)}, "50 100 75"},
		{[]any{of("a", 5, 8), of("b", 3, 3), of("c", 0, 5), of("d", 1, 2.5)}, "62.5 100 0 40"},
		{[]any{of("a", 2, nil), item(4)}, "50 100"},
		{[]any{of("a", 1, 0)}, `bars: bar 1 "a"'s of takes a number above 0, not 0`},
		{[]any{of("a", 1, -2)}, `bars: bar 1 "a"'s of takes a number above 0, not -2`},
		{[]any{of("a", 1, "8")}, `bars: bar 1 "a"'s of takes a number, not "8"`},
		{[]any{of("a", 5, 3), of("b", 1, 2), of("c", 9, 4)}, `bars: bar 1 "a": 5 is over its of 3; write a value no greater than of, the whole` + "\n" +
			`bars: bar 3 "c": 9 is over its of 4; write a value no greater than of, the whole`},
		// Past 2^53, a float holds both as one number: they are compared as integers.
		{
			[]any{of("a", 9007199254740993, 9007199254740992)},
			`bars: bar 1 "a": 9,007,199,254,740,993 is over its of 9,007,199,254,740,992; write a value no greater than of, the whole`,
		},
		{[]any{of("a", 3e21, 2e21)}, `bars: bar 1 "a": 3,000,000,000,000,000,000,000 is over its of 2,000,000,000,000,000,000,000; write a value no greater than of, the whole`},
		// No label: told by the number.
		{[]any{item(1), map[string]any{"value": 2, "of": 1}}, `bars: bar 2 "": 2 is over its of 1; write a value no greater than of, the whole`},
	} {
		bars, err := barsOf(c.items)
		widths := make([]string, 0, len(bars))
		for _, b := range bars {
			widths = append(widths, fmt.Sprint(b.Width))
		}
		got := strings.Join(widths, " ")
		if err != nil {
			got = usererr.Tell(i18n.EnglishCatalog(), err).Render("")
		}
		if got != c.want {
			t.Errorf("barsOf(%v) = %q; want %q", c.items, got, c.want)
		}
	}
}

// Now and Max, a bar's value and of for aria, are plain digits as num writes them, without its commas: a whole
// number beyond 2^53 keeps its digits, and a large or small float has no exponent.
func TestBarsAria(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		value, of any
		want      string
	}{
		{9007199254740993, 9007199254740995, "9007199254740993 9007199254740995"},
		{1e21, 2e21, "1000000000000000000000 2000000000000000000000"},
		{1e-7, 2e-7, "0.0000001 0.0000002"},
		{1500000, 2000000, "1500000 2000000"},
	} {
		bars, err := barsOf([]any{map[string]any{"label": "x", "value": c.value, "of": c.of}})
		if err != nil {
			t.Fatal(err)
		}
		if got := bars[0].Now + " " + bars[0].Max; got != c.want {
			t.Errorf("barsOf(%v of %v) = %q; want %q", c.value, c.of, got, c.want)
		}
	}
}

// A code's lines keep their text, and mark counts as the numbers shown do: from start, or from 1 without it.
func TestCodeOf(t *testing.T) {
	t.Parallel()

	line := func(text string) codeLine { return codeLine{Text: text, Mark: false} }
	marked := func(text string) codeLine { return codeLine{Text: text, Mark: true} }
	for _, tt := range []struct {
		name        string
		text        string
		start, mark any
		want        codeText
	}{
		{"plain", "a\nb\n", nil, nil, codeText{Lines: []codeLine{line("a\n"), line("b\n")}, Before: 0, Digits: 1, Numbered: false}},
		{"no last newline", "a\nb", nil, nil, codeText{Lines: []codeLine{line("a\n"), line("b")}, Before: 0, Digits: 1, Numbered: false}},
		{"blank lines", "\na\n\n", nil, nil, codeText{Lines: []codeLine{line("\n"), line("a\n"), line("\n")}, Before: 0, Digits: 1, Numbered: false}},
		{"CRLF", "a\r\nb\r\n", nil, nil, codeText{Lines: []codeLine{line("a\n"), line("b\n")}, Before: 0, Digits: 1, Numbered: false}},
		{"CR", "a\rb", nil, nil, codeText{Lines: []codeLine{line("a\n"), line("b")}, Before: 0, Digits: 1, Numbered: false}},
		{"mark from 1", "a\nb\n", nil, []any{2}, codeText{Lines: []codeLine{line("a\n"), marked("b\n")}, Before: 0, Digits: 1, Numbered: false}},
		{
			"mark from start", "a\nb\nc\n", 12, []any{13},
			codeText{Lines: []codeLine{line("a\n"), marked("b\n"), line("c\n")}, Before: 11, Digits: 2, Numbered: true},
		},
		{
			"a number of the text's count marks nothing under start", "a\nb\n", 12, []any{1, 99},
			codeText{Lines: []codeLine{line("a\n"), line("b\n")}, Before: 11, Digits: 2, Numbered: true},
		},
		{
			"read from YAML as float or uint64", "a\nb\n", 5.0, []any{6.0, uint64(5)},
			codeText{Lines: []codeLine{marked("a\n"), marked("b\n")}, Before: 4, Digits: 1, Numbered: true},
		},
		{
			"digits of the last line", "a\nb\n", 9, nil,
			codeText{Lines: []codeLine{line("a\n"), line("b\n")}, Before: 8, Digits: 2, Numbered: true},
		},
		{
			"the largest number", "a\n", 1_000_000_000, []any{1_000_000_000},
			codeText{Lines: []codeLine{marked("a\n")}, Before: 999_999_999, Digits: 10, Numbered: true},
		},
	} {
		got, err := codeOf(tt.text, tt.start, tt.mark)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// What a part of one's own hands code is refused whole: floe's part has its schema say so first.
func TestCodeOfRefuses(t *testing.T) {
	t.Parallel()

	line := func(key, value string) string {
		return "code's " + key + " takes a whole number from 1 to 1,000,000,000, not " + value
	}
	for _, tt := range []struct {
		start, mark any
		want        string
	}{
		{0, nil, line("start", "0")},
		{-1, nil, line("start", "-1")},
		{12.5, nil, line("start", "12.5")},
		{2_000_000_000, nil, line("start", "2000000000")},
		{"a", nil, `code's start takes a number, not "a"`},
		{nil, 2, "code's mark takes a list of line numbers, not 2"},
		{nil, "2", "code's mark takes a list of line numbers, not 2"},
		{nil, []any{1.5}, line("mark", "1.5")},
		{nil, []any{0}, line("mark", "0")},
		{nil, []any{2_000_000_000}, line("mark", "2000000000")},
		{nil, []any{"1"}, `code's mark takes a number, not "1"`},
		{nil, []any{0, 2.5}, line("mark", "0") + "\n" + line("mark", "2.5")},
	} {
		_, err := codeOf("a\n", tt.start, tt.mark)
		if got := strings.TrimSpace(usererr.Tell(i18n.EnglishCatalog(), err).Render("")); got != tt.want {
			t.Errorf("start %v, mark %v: told %q, want %q", tt.start, tt.mark, got, tt.want)
		}
	}
}
