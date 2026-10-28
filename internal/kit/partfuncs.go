package kit

import (
	_ "embed" // the copy button's script
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// partFuncs are the functions floe's parts draw with, besides those of page.html.
var partFuncs = template.FuncMap{
	"bars": barsOf, "copyButton": copyButtonOf, "diagram": diagramOf, "diff": diffOf, "script": scriptOf,
	"tree": treeOf,
}

// copyButtonOf holds the name of the function that draws the copy button of a part, which Part.Page binds anew in
// the language of the page it draws.
func copyButtonOf() (template.HTML, error) {
	return "", fmt.Errorf("%w: copyButton: not bound to a page", ErrFunc)
}

//go:embed copy.js
var copyJS string

// copyButton returns the function copyButton of a part.html drawn in the language of c: the script that puts a
// button over the code of the part it is in (copy.js), with its words.
// LIMIT: the script goes into each part that copies. Put it into the page once if pages of much code come to weigh.
func copyButton(c *i18n.Catalog) func() (template.HTML, error) {
	words := map[string]string{"copy": c.Msg("copy_button"), "copied": c.Msg("ui_copied"), "failed": c.Msg("copy_failed")}
	return func() (template.HTML, error) { return copyScript(words) }
}

// copyScript returns the script of copy.js called with words. json.Marshal writes <, > and & as \u escapes, so
// that no word ends the script.
func copyScript(words map[string]string) (template.HTML, error) {
	b, err := json.Marshal(words)
	if err != nil {
		return "", fmt.Errorf("%w: copyButton: %w", ErrFunc, err)
	}
	//nolint:gosec // G203: floe's script, the words escaped as JSON
	return template.HTML("<script>(" + strings.TrimSpace(copyJS) + ")(" + string(b) + ");</script>"), nil
}

// diagramOf holds the name of the function that draws the value of the part as a diagram, which draw binds
// anew for each part it draws: what it draws with (the page's language, the icons, the diagram's number in
// the page) is the page's.
func diagramOf() (template.HTML, error) {
	return "", fmt.Errorf("%w: diagram: not bound to a page", ErrFunc)
}

// bar is an item of a bars part, and the width of its bar, a percentage of the longest's.
type bar struct {
	Label, Tone string
	Value       any
	Width       float64
}

// barsOf gives each of the items of a bars part, each a label and a value no less than 0, its width.
func barsOf(items []any) ([]bar, error) {
	bars := make([]bar, 0, len(items))
	longest := 0.0
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		n, err := float("bars", m["value"])
		if err != nil {
			return nil, err
		}
		longest = max(longest, n)
		bars = append(bars, bar{Label: stringOf(m["label"]), Tone: stringOf(m["tone"]), Value: m["value"], Width: n})
	}
	for i := range bars {
		if longest > 0 {
			bars[i].Width = bars[i].Width / longest * 100
		}
	}
	return bars, nil
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
		return "", usererr.New("kit_script_end")
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
