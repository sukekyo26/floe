package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/usererr"
)

// partFuncs are the functions floe's parts draw with, besides those of page.html.
var partFuncs = template.FuncMap{"bars": barsOf, "diff": diffOf, "script": scriptOf, "tree": treeOf}

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

// diffLine is a line of a diff and its kind, the class it is drawn in: add, del, hunk (@@), meta (the
// lines naming the files, \ No newline at end of file), or "" for a line that stays.
type diffLine struct {
	Kind, Text string
}

// diffText is a diff by lines, with the counts of the lines it adds and removes.
type diffText struct {
	Lines          []diffLine
	Added, Removed int
}

// diffOf reads text, a unified diff as git diff writes it or lines of it, each line keeping its newline.
// Outside a hunk, a line not starting with +, - or a space (diff --git, index) ends the hunk before it,
// and --- and +++ name the files.
func diffOf(text string) diffText {
	var d diffText
	inHunk := false
	for line := range strings.Lines(text) {
		kind := ""
		switch {
		case strings.HasPrefix(line, "@@"):
			kind, inHunk = "hunk", true
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
		d.Lines = append(d.Lines, diffLine{Kind: kind, Text: line})
	}
	return d
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
