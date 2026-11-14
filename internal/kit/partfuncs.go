package kit

import (
	_ "embed" // the copy button's script
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"floe/internal/i18n"
	"floe/internal/page"
	"floe/internal/usererr"
	"floe/internal/zoom"
)

// partFuncs are the functions floe's parts draw with, besides those of page.html and over them: markdown draws in a
// text's box, as all a part holds of Markdown is (MarkdownFlow).
var partFuncs = template.FuncMap{
	"copyButton": copyButtonOf, "diff": diffOf, "markdown": MarkdownFlow, "tree": treeOf, "zoom": zoomScript,
}

// zoomScript is the function zoom of a part.html: the script that zooms the frames of class floe-zoom, as the
// diagram has it.
// LIMIT: the script goes into each part that zooms, as the copy button's does (some 7 KB each). Put it into the page
// once if pages of many images come to weigh.
func zoomScript() (template.HTML, error) {
	s, err := zoom.Script()
	if err != nil {
		return "", fmt.Errorf("%w: zoom: %w", ErrFunc, err)
	}
	return s, nil
}

// copyButtonOf holds the name of the function that draws the copy button of a part, which Part.Page binds anew in
// the language of the page it draws.
func copyButtonOf() (template.HTML, error) {
	return "", fmt.Errorf("%w: copyButton: not bound to a page", ErrFunc)
}

//go:embed contents.js
var contentsJS string

// contentsScript returns the script that marks, in the contents just before it, the place a page is read at.
func contentsScript() (template.HTML, error) {
	script, err := page.Script(contentsJS, nil)
	if err != nil {
		return "", fmt.Errorf("%w: contents: %w", ErrFunc, err)
	}
	//nolint:gosec // G203: floe's script, of no words
	return template.HTML(script), nil
}

//go:embed copy.js
var copyJS string

// copyScripts are the scripts of copy.js, that put a button over the code of the part they are in, by the languages
// with a catalog, each with its words.
// LIMIT: the script goes into each part that copies. Put it into the page once if pages of much code come to weigh.
// LIMIT: HTML reads CR and CRLF as LF, so a diff of a CRLF file is copied with LF and does not apply. Keep the text in
// an attribute too if diffs of CRLF files come to be copied.
// LIMIT: the copy command is called after writeText is refused, not in the click itself; Chromium lets it copy then
// (the click's activation lasts), Firefox is not tried. Call it in the click where a browser comes to refuse it.
var copyScripts = sync.OnceValues(func() (map[i18n.Lang]template.HTML, error) {
	scripts := map[i18n.Lang]template.HTML{}
	for _, lang := range i18n.Langs() {
		c := i18n.New(lang)
		words := map[string]string{"copy": c.Msg("copy_button"), "copied": c.Msg("ui_copied"), "failed": c.Msg("copy_failed")}
		script, err := page.Script(copyJS, words)
		if err != nil {
			return nil, fmt.Errorf("%w: copyButton: %w", ErrFunc, err)
		}
		//nolint:gosec // G203: floe's script, its words escaped as JSON
		scripts[lang] = template.HTML(script)
	}
	return scripts, nil
})

// copyButton returns the function copyButton of a part.html drawn in a page of the language tag: the script of
// copyScripts in the first of the tag's fallbacks with a catalog, English at the last.
func copyButton(tag string) func() (template.HTML, error) {
	return func() (template.HTML, error) {
		scripts, err := copyScripts()
		if err != nil {
			return "", err
		}
		for _, lang := range i18n.Fallbacks(i18n.Tag(tag)) {
			if script, ok := scripts[lang]; ok {
				return script, nil
			}
		}
		return "", fmt.Errorf("%w: copyButton: no script in English", ErrFunc)
	}
}

// bar is an item of a bars part, and the width of its bar, a percentage of its whole: of when written, else the
// largest value of the bars without of. Now and Max are its value and of in plain digits, for aria.
type bar struct {
	Label, Tone, Unit string
	Value, Of         any
	Now, Max          string
	Width             float64
}

// barsOf gives each of the items of a bars part, each written as keys and values, a label, a value no less than 0
// and an of above 0 and no less than the value, or none (nil: Walk sets a key the schema declares and the data
// leaves out to nil, as null), its width. It tells the mistakes of every item, each at the key of the item it is of.
// LIMIT: an item with of and no label, which floe's bars refuses by its schema, is a progressbar of no name; refuse
// it here if a part of one's own comes to draw one.
// LIMIT: a unit is a bar's own, so a bar without of, whose width is a share of the longest value, is not refused for a
// unit its neighbours do not share: bars of different units compared by length mean nothing. Bars with of, each its own
// whole, may mix them (pages and minutes). Refuse the mix if a comparison that misleads comes to be written.
func barsOf(items []any) ([]bar, error) {
	bars := make([]bar, 0, len(items))
	var found usererr.List
	longest := 0.0
	for i, item := range items {
		b, key, err := barOf(i, item)
		if err != nil {
			at := "#/" + strconv.Itoa(i)
			if key != "" {
				at += "/" + key
			}
			if failed := found.Add(placed(err, at)); failed != nil {
				return nil, failed
			}
			continue
		}
		if b.Of == nil {
			longest = max(longest, b.Width)
		}
		bars = append(bars, b)
	}
	if found != nil {
		return nil, found.Err()
	}
	for i := range bars {
		if bars[i].Of == nil && longest > 0 {
			bars[i].Width = bars[i].Width / longest * 100
		}
	}
	return bars, nil
}

// barOf reads the item at i of a bars part: its width is its value, or a percentage of its of, when written. A
// mistake comes with the key of the item it is of, "" for the item itself.
func barOf(i int, item any) (bar, string, error) {
	m, ok := item.(map[string]any)
	if !ok {
		return bar{}, "", usererr.New("kit_bar_item", "bars", i+1)
	}
	b := bar{
		Label: stringOf(m["label"]), Tone: stringOf(m["tone"]), Unit: stringOf(m["unit"]),
		Value: m["value"], Of: m["of"], Now: "", Max: "", Width: 0,
	}
	name := fmt.Sprintf("bars: bar %d %q", i+1, b.Label)
	// A unit of a part of one's own, whose schema does not say it is text, would be read as none by stringOf.
	if _, text := m["unit"].(string); !text && m["unit"] != nil {
		return bar{}, "unit", usererr.New("kit_not_text", name+"'s unit", fmt.Sprint(m["unit"]))
	}
	n, err := float(name+"'s value", b.Value)
	if err != nil {
		return bar{}, "value", err
	}
	// Its width is a share: one below 0 would be no width, which CSS drops, drawing the bar whole.
	if n < 0 {
		return bar{}, "value", usererr.New("kit_below_zero", name+"'s value", fmt.Sprint(b.Value))
	}
	b.Width = max(n, 0) // -0, which n < 0 lets through, as 0
	if b.Of == nil {
		return b, "", nil
	}
	of, err := float(name+"'s of", b.Of)
	if err != nil {
		return bar{}, "of", err
	}
	// An of of 0 would make the width infinite, one below 0 below 0, and one below the value wider than the track.
	if of <= 0 {
		return bar{}, "of", usererr.New("kit_bar_of", name+"'s of", fmt.Sprint(b.Of))
	}
	if over(b.Value, b.Of, n, of) {
		var value, whole string
		if value, err = decimal(name+"'s value", b.Value, 0); err != nil {
			return bar{}, "value", err
		}
		if whole, err = decimal(name+"'s of", b.Of, 0); err != nil {
			return bar{}, "of", err
		}
		// The value is what to lower, as the mistake says.
		return bar{}, "value", usererr.New("kit_bar_over", name, value, whole)
	}
	b.Width = b.Width / of * 100
	if b.Now, err = plain(name+"'s value", b.Value, 0); err != nil {
		return bar{}, "value", err
	}
	if b.Max, err = plain(name+"'s of", b.Of, 0); err != nil {
		return bar{}, "of", err
	}
	return b, "", nil
}

// over is whether a bar's value v, read as n, goes past its of w, read as of: compared as integers when both are
// whole, which a float rounds past 2^53.
func over(v, w any, n, of float64) bool {
	if a, ok := whole(v); ok {
		if b, ok := whole(w); ok {
			return a.Cmp(b) > 0
		}
	}
	return n > of
}

// diffLine is a line of a diff and its kind, the class it is drawn in: add, del, hunk (@@), meta (outside a
// hunk, the lines naming the files and the others, such as diff --git and index; \ No newline at end of
// file), or "" for a line that stays.
type diffLine struct {
	Kind, Text string
}

// diffText is a diff by lines, with the counts of the lines it adds and removes.
type diffText struct {
	Lines          []diffLine
	Added, Removed int
}

// diffOf reads text, a unified diff as git diff writes it or lines of it, each line keeping its newline.
// A hunk ends when the lines its head counts on both sides are used up, or at a line not starting with +,
// -, a space or \ (diff --git, index); outside a hunk, --- and +++ name the files.
// LIMIT: a + or - line outside a hunk is counted as added or removed, so that lines without their head
// draw, and so are git format-patch's --- before the diffstat and its -- signature; and the next file's
// --- and +++ right after a hunk shorter than its head are read as removed and added. Read the mail's lines
// apart when a patch is drawn.
func diffOf(text string) diffText {
	var d diffText
	inHunk, counted := false, false
	var left hunkLeft
	for line := range strings.Lines(text) {
		kind := ""
		switch {
		case strings.HasPrefix(line, "@@"):
			kind, inHunk = "hunk", true
			left, counted = hunkLengths(line)
		case strings.HasPrefix(line, `\`):
			kind = "meta"
		case !inHunk && (strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ")):
			kind = "meta"
		case strings.HasPrefix(line, "+"):
			kind = "add"
			d.Added++
		case strings.HasPrefix(line, "-"):
			kind = "del"
			d.Removed++
		case strings.HasPrefix(line, " ") || strings.TrimSpace(line) == "":
		default:
			kind, inHunk = "meta", false
		}
		if inHunk && counted && left.take(kind) {
			inHunk = false
		}
		d.Lines = append(d.Lines, diffLine{Kind: kind, Text: line})
	}
	return d
}

// codeLine is a line of a code part, keeping its newline, and whether the part marks it.
type codeLine struct {
	Text string
	Mark bool
}

// codeText is a code part's text by lines. Numbered is whether start is written; Before is the count before the first
// line, and Digits the digits of the last line's number.
type codeText struct {
	Lines          []codeLine
	Before, Digits int
	Numbered       bool
}

// newlines makes CR and CRLF LF before the lines are cut, as HTML does to what it reads: else what a CR alone breaks
// would draw as two lines under one number.
var newlines = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// codeOf cuts text into lines, each keeping its newline, numbered from start, or from 1 when it is nil, and marks the
// lines mark, nil or a list, names by those numbers. It tells every wrong number of mark.
// LIMIT: a number no line has marks nothing, and a start changed alone moves what mark names: the page shows both. Tell
// them apart here, which counts the lines, if the page comes not to be looked at.
// LIMIT: the range of a number is the schema's, as the part code's is. A part of one's own with no such schema may
// number from 0, or past a CSS counter's end near 2^31, where its lines would all show one number: the page shows it.
// A number past an int (start: 18446744073709551615) reads as another, and a start below 0 may show more digits than
// Digits, which is the last line's: floe's part has its schema refuse both.
func codeOf(text string, start, mark any) (codeText, error) {
	c := codeText{Lines: nil, Before: 0, Digits: 0, Numbered: false}
	var found usererr.List
	first := 1
	if start != nil {
		n, err := lineNumber("start", start)
		if failed := found.Add(placed(err, "#/start")); failed != nil {
			return c, failed
		}
		first, c.Numbered = n, true
	}
	marks := map[int]bool{}
	list, isList := mark.([]any)
	if mark != nil && !isList {
		if failed := found.Add(usererr.New("kit_code_mark", fmt.Sprint(mark)).PlacedIn("#/mark")); failed != nil {
			return c, failed
		}
	}
	for k, v := range list {
		n, err := lineNumber("mark", v)
		if failed := found.Add(placed(err, "#/mark/"+strconv.Itoa(k))); failed != nil {
			return c, failed
		}
		marks[n] = true
	}
	if found != nil {
		return c, found.Err()
	}
	n := first
	for line := range strings.Lines(newlines.Replace(text)) {
		c.Lines = append(c.Lines, codeLine{Text: line, Mark: marks[n]})
		n++
	}
	c.Before, c.Digits = first-1, len(strconv.Itoa(n-1))
	return c, nil
}

// lineNumber reads v as the number of a line: a whole number, of the kinds YAML reads one as.
func lineNumber(key string, v any) (int, error) {
	if i, ok := whole(v); ok {
		return int(i.Int64()), nil
	}
	if f, ok := v.(float64); ok && f == math.Trunc(f) {
		return int(f), nil
	}
	return 0, usererr.New("kit_code_line", key, fmt.Sprint(v))
}

// hunkHead is the start of a hunk's head, @@ -a,b +c,d @@, b and d its lengths, 1 when left out.
var hunkHead = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// hunkLeft is what is left of a hunk's lengths, the lines it has on the old side and on the new.
type hunkLeft struct {
	old, added int
}

// take takes a line of kind from what is left, and reports whether both sides are used up.
func (h *hunkLeft) take(kind string) bool {
	switch kind {
	case "add":
		h.added--
	case "del":
		h.old--
	case "":
		h.old--
		h.added--
	}
	return h.old <= 0 && h.added <= 0
}

// hunkLengths returns the lengths of the hunk whose head is line, and false for a head that does not give
// them (@@ alone, a combined diff's @@@, a length too large for an int).
func hunkLengths(line string) (hunkLeft, bool) {
	m := hunkHead.FindStringSubmatch(line)
	if m == nil {
		return hunkLeft{old: 0, added: 0}, false
	}
	n := [2]int{1, 1}
	for i, s := range m[1:] {
		if s == "" {
			continue
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return hunkLeft{old: 0, added: 0}, false
		}
		n[i] = v
	}
	return hunkLeft{old: n[0], added: n[1]}, true
}

// treeNode is a file or a folder of a tree, with what is in a folder in the order the paths name it.
type treeNode struct {
	Name, Note, Change string
	Dir                bool
	Children           []*treeNode
}

// treeOf nests the items of a tree part, each a path (a folder's ending in /) with its note and change,
// into the folders the paths name, those not listed of their own included: a/b.go and a/c/ make a/
// holding b.go and c/. A path listed again, or a folder listed after a path in it, gives the one drawn
// its note and change.
func treeOf(items []any) []*treeNode {
	root := &treeNode{Name: "", Note: "", Change: "", Dir: true, Children: nil}
	byPath := map[string]*treeNode{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		p := stringOf(m["path"])
		names := strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
		parent, at := root, ""
		for i, name := range names {
			dir := i < len(names)-1 || strings.HasSuffix(p, "/")
			at += name
			if dir {
				at += "/"
			}
			node := byPath[at]
			if node == nil {
				node = &treeNode{Name: name, Note: "", Change: "", Dir: dir, Children: nil}
				if dir {
					node.Name += "/"
				}
				byPath[at] = node
				parent.Children = append(parent.Children, node)
			}
			parent = node
		}
		if parent != root {
			parent.Note, parent.Change = stringOf(m["note"]), stringOf(m["change"])
		}
	}
	return root.Children
}

// stringOf returns v if it is a string, else "": an item the data leaves out is nil.
func stringOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// scriptOf draws a script part: an element to draw in, and a script that calls js, a function, with it and data.
// js is assigned, so that a ; or a comment after the function still parses, and the line break ends a //
// comment before the call. data goes in as the text of its JSON, whose < and > are escaped, so that no text of
// it ends the script, and is parsed, so that a key __proto__ is a key. The script is refused when an HTML
// tokenizer does not read it as one text ended by </script> (a </script or a <!-- before a <script in js).
func scriptOf(js string, data any) (template.HTML, error) {
	// Values read from YAML, without .inf or .nan or keys other than text, always encode.
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("%w: script: %w", ErrFunc, err)
	}
	text, err := json.Marshal(string(b))
	if err != nil {
		return "", fmt.Errorf("%w: script: %w", ErrFunc, err)
	}
	body := "{const floeScript = " + js + "\n" +
		";floeScript(document.currentScript.previousElementSibling, JSON.parse(" + string(text) + "));}"
	if !wholeScript(body) {
		return "", usererr.New("kit_script_end").PlacedIn("#/js")
	}
	//nolint:gosec // G203: read whole as a script's text
	return template.HTML("<div></div><script>" + body + "</script>"), nil
}

// wholeScript reports whether an HTML tokenizer reads <script>body</script> as the tag, body as one text, and
// </script> as written. Raw, not Text, which makes CRLF LF.
func wholeScript(body string) bool {
	z := html.NewTokenizer(strings.NewReader("<script>" + body + "</script>"))
	want := []struct {
		typ html.TokenType
		raw string
	}{{html.StartTagToken, "<script>"}, {html.TextToken, body}, {html.EndTagToken, "</script>"}}
	for _, w := range want {
		if z.Next() != w.typ || string(z.Raw()) != w.raw {
			return false
		}
	}
	return z.Next() == html.ErrorToken && errors.Is(z.Err(), io.EOF)
}
