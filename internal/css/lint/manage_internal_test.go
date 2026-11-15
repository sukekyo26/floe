package lint

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"

	floecss "floe/internal/css"
)

// uiSheet is a piece of the management page's CSS: a <style>, or an element's style attribute written as a rule of its
// own, with the line of ui.html before it starts (a style attribute's: before its element's tag).
type uiSheet struct {
	where string // what it is, for a finding: "" for a <style>, "<tag style>" for an attribute
	line  int
	src   []byte
}

// uiAttr is an SVG attribute that takes a colour, its element and the line of its tag.
type uiAttr struct {
	tag, key, val string
	line          int
}

// uiParts returns the management page's CSS and its SVG attributes that take a colour, read by x/net/html's tokenizer,
// which tells where each token starts.
func uiParts(ui []byte) ([]uiSheet, []uiAttr, error) {
	var sheets []uiSheet
	var attrs []uiAttr
	z := html.NewTokenizer(bytes.NewReader(ui))
	at, style := 0, false
	for {
		tt := z.Next()
		raw := len(z.Raw())
		line := lineOf(ui, at) - 1
		switch tt {
		case html.ErrorToken:
			if err := z.Err(); !errors.Is(err, io.EOF) {
				return nil, nil, fmt.Errorf("ui.html: %w", err)
			}
			return sheets, attrs, nil
		case html.TextToken:
			if style {
				sheets = append(sheets, uiSheet{where: "", line: line, src: bytes.Clone(z.Text())})
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			style = tok.Data == "style" && tt == html.StartTagToken
			for _, a := range tok.Attr {
				if slices.Contains(svgColors, a.Key) {
					attrs = append(attrs, uiAttr{tok.Data, a.Key, a.Val, line + 1})
				}
				if a.Key == "style" {
					sheets = append(sheets, uiSheet{where: "<" + tok.Data + " style>", line: line, src: []byte("style {" + a.Val + "}")})
				}
			}
		default:
			style = false
		}
		at += raw
	}
}

// uiFinding is a finding of the rule at the byte at of s.
func uiFinding(rule string, s uiSheet, at int, text string) Finding {
	if s.where != "" {
		return Finding{"ui.html", s.line + 1, rule, s.where + ": " + text}
	}
	return Finding{"ui.html", s.line + lineOf(s.src, at), rule, text}
}

// bareRoot is whether rule sets a declaration on :root itself, unnarrowed (by @media and the like alone): where the
// management page's steps and colours are.
func bareRoot(rule string) bool {
	preludes := strings.Split(rule, " { ")
	return rootOf(rule, ":root") == onTheRoot && strings.TrimSpace(preludes[len(preludes)-1]) == ":root"
}

// colorFunctions are the functions that make a colour.
var colorFunctions = []string{
	"rgb(", "rgba(", "hsl(", "hsla(", "hwb(", "lab(", "lch(", "oklab(", "oklch(", "color(", "color-mix(", "light-dark(",
}

// svgColors are the SVG attributes that take a colour.
var svgColors = []string{"fill", "stroke", "stop-color", "flood-color", "lighting-color", "color"}

// colored reports whether a value writes a colour of its own: a # colour or a colour function.
func colored(value []token) bool {
	return slices.ContainsFunc(value, func(t token) bool {
		return t.Type == css.HashToken || t.Type == css.FunctionToken && slices.Contains(colorFunctions, strings.ToLower(string(t.Data)))
	})
}

// manageColours keeps the management page's (ui.html's) colours in its :root alone: a value outside :root writes no
// colour of its own (a # colour or a colour function), in its style and in an element's style attribute, nor does an SVG
// attribute of its markup. An exception's Key is a rule's prelude, whose value mixes the colours of :root (color-mix) and
// writes no other colour.
// LIMIT: a colour's name (white) is not read, nor the HTML the script builds in its strings.
var manageColours = Rule{Name: "manage-colours", Check: func(in *Input) []Finding {
	sheets, attrs, err := uiParts(in.UI())
	if err != nil {
		return []Finding{{"ui.html", 0, "manage-colours", err.Error()}}
	}
	var found []Finding
	for _, a := range attrs {
		if colored(tokensOf([]byte(a.val))) {
			found = append(found, Finding{"ui.html", a.line, "manage-colours", "<" + a.tag + " " + a.key + "=\"" + a.val + "\">: give the colour a variable in :root and set it from the style"})
		}
	}
	for _, s := range sheets {
		all := declarations(s.src, func(string) bool { return true })
		for _, name := range slices.Sorted(maps.Keys(all)) {
			for _, d := range all[name] {
				in.Saw()
				if bareRoot(d.rule) {
					continue
				}
				value := d.value
				if in.Allowed(d.rule) {
					value = slices.DeleteFunc(slices.Clone(value), func(t token) bool {
						return t.Type == css.FunctionToken && strings.EqualFold(string(t.Data), "color-mix(")
					})
				}
				if colored(value) {
					found = append(found, uiFinding("manage-colours", s, d.at, name+": "+text(s.src, d.value)+" in "+d.rule+"; give the colour a variable in :root"))
				}
			}
		}
	}
	return found
}, Allow: []Allow{
	{".proj-tint", "a project's colour is each row's, which :root cannot know: mixed with :root's colours outside it"},
}}

// manageCorners are the management page's corners: its three steps, a box within a pop-up's or a menu's box (the outer
// corner less the room between them), and the shapes.
var manageCorners = slices.Concat([]string{
	"var(--corner)", "var(--corner-menu)", "var(--corner-small)",
	"calc(var(--corner) - var(--menu-pad))", "calc(var(--corner-menu) - var(--menu-pad))",
}, cornerShapes)

// manageSteps are the steps :root sets, which no other rule sets again.
var manageSteps = []string{"--corner", "--corner-menu", "--corner-small", "--menu-pad", "--text-label", "--text-small", "--text"}

// manageText are the management page's steps of text; a heading of 1rem or more and an em of 1 or more are written as
// numbers.
var manageText = []string{"var(--text-label)", "var(--text-small)", "var(--text)", "inherit", "0"}

// radiusProperty names the properties of corners.
var radiusProperty = regexp.MustCompile(`^border(-[a-z]+)*-radius$`)

// manageCornersRule keeps the management page's corners and text to the steps its :root sets, set nowhere else. An
// exception's Key is a rule's prelude and the size it writes (".x code: 0.8em").
// LIMIT: corners are read in border-…-radius alone, not in -webkit-border-radius nor clip-path's inset(… round …).
var manageCornersRule = Rule{Name: "manage-corners", Check: func(in *Input) []Finding {
	sheets, _, err := uiParts(in.UI())
	if err != nil {
		return []Finding{{"ui.html", 0, "manage-corners", err.Error()}}
	}
	var found []Finding
	for _, s := range sheets {
		all := declarations(s.src, func(string) bool { return true })
		for _, name := range slices.Sorted(maps.Keys(all)) {
			for _, d := range all[name] {
				in.Saw()
				value := text(s.src, d.value)
				switch {
				case slices.Contains(manageSteps, name) && !bareRoot(d.rule):
					found = append(found, uiFinding("manage-corners", s, d.at, "sets "+name+" in "+d.rule+"; only :root sets the steps"))
				case radiusProperty.MatchString(name):
					for _, c := range floecss.Items(d.value) {
						if c := text(s.src, c); c != "/" && !slices.Contains(manageCorners, c) {
							found = append(found, uiFinding("manage-corners", s, d.at, name+": "+value+" in "+d.rule+"; read one of :root's corners "+strings.Join(manageCorners, ", ")))
						}
					}
				case name == "font-size" || name == "font":
					size, ok := sizeOf(name, d.value)
					if !ok || !slices.Contains(manageText, size) && !large(size) && !in.Allowed(d.rule+": "+size) {
						found = append(found, uiFinding("manage-corners", s, d.at, name+": "+value+" in "+d.rule+"; read one of :root's steps of text "+strings.Join(manageText, ", ")))
					}
				}
			}
		}
	}
	return found
}}
