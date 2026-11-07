package css_test

import (
	"bytes"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/css"
	"github.com/sukekyo26/floe/internal/kit"
)

// switching are, by part, the variables it sets on elements in it rather than on its root: its own, switched between
// the elements or made from the data on each (a style attribute, which wins over a theme). No theme sets them. Add
// one only with what it switches between.
var switching = map[string]map[string]string{
	"chart": {
		"--c":            "the colour of each series (.s1 to .s4)",
		"--left":         "the room for the left axis, from the data (the svg's style)",
		"--right":        "the room for the right axis, from the data (the svg's style)",
		"--right-narrow": "the room for the right axis on a narrow screen, from the data (the svg's style)",
	},
	"code": {
		"--floe-code-gutter": "the room for the line numbers, with them (.numbered) or without (.lines)",
		"--digits":           "the digits of the last line's number, from the data (the style of <code class=\"lines\">)",
	},
	"zoom": {"--floe-tool-alpha": "a button's colours, faded while it is disabled"},
}

// floesOwn are the variables of floe's own ways, set by rules of their own, out of this test: named here, not read
// from the files the test checks, so that a file setting one more cannot take a variable out of it.
var floesOwn = func() map[string]string {
	own := map[string]string{
		"--face":              "a face's ground (css.md: a painted face), base.css clears it on a part's box",
		"--on-face":           "a face's text, as --face",
		"--floe-pill-wrap":    "a pill's wrapping, base.css clears it on a part's box and tags sets it on its tags",
		"--tone":              "a tone's colour, which floe builds from tone.json (css.md: tone)",
		"--tone-soft":         "a tone's soft colour, as --tone",
		"--tone-fill":         "a tone's fill, as --tone",
		"--floe-shadow-color": "a box's shadow, which base.css registers to read on each box alone",
		"--floe-inset":        "a box's inset, as --floe-shadow-color",
	}
	for _, v := range []string{"--floe-copy-gap", "--floe-copy-room", "--floe-copy-size", "--floe-pad-block", "--floe-pad-inline", "--floe-pill-end", "--floe-room", "--floe-row-pad", "--floe-text-aside", "--floe-text-small"} {
		own[v] = "floe's base, set on the root of base.css"
	}
	for _, v := range []string{
		"--accent", "--accent-soft", "--bad", "--bad-soft", "--bg", "--chart-1", "--chart-2", "--chart-3", "--chart-4", "--code-add", "--code-bg", "--code-del", "--code-fg", "--code-prompt", "--display", "--fg", "--font", "--good", "--good-soft", "--line", "--mono", "--muted", "--muted-soft", "--on-fill", "--page-bg", "--page-pad", "--page-width", "--quiet", "--radius", "--radius-pill", "--radius-small", "--rule", "--shadow", "--surface", "--text-width", "--warn", "--warn-soft",
	} {
		own[v] = "the palette, set on the root of default's palette.css and a theme's"
	}
	return own
}()

// everyPart is the page of every part as floe draws it, which TestCorpus holds to what floe draws now.
const everyPart = "../corpus/testdata/every-part/golden/every-part.html"

// drawn returns, by part, the variables the style attributes of every-part set in it, and the CSS of the page, read
// by x/net/html.
// LIMIT: a style attribute every-part's data does not draw goes unread; add the data to every-part if a part comes to
// set a variable so.
func drawn(t *testing.T) (map[string]map[string]bool, []byte) {
	t.Helper()

	src, err := os.ReadFile(everyPart)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	styles, sheet := map[string]map[string]bool{}, &bytes.Buffer{}
	var walk func(n *html.Node, part string)
	walk = func(n *html.Node, part string) {
		for _, a := range n.Attr {
			if a.Key == "data-floe-part" {
				part = a.Val
			}
		}
		for _, a := range n.Attr {
			if a.Key == "style" {
				if styles[part] == nil {
					styles[part] = map[string]bool{}
				}
				for v := range css.Settings([]byte(a.Val)) {
					styles[part][v] = true
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "style" && n.FirstChild != nil {
			sheet.WriteString(n.FirstChild.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, part)
		}
	}
	walk(doc, "")
	return styles, sheet.Bytes()
}

// TestPartVariables keeps a part's variables where a theme can set them. A part or a look sets a variable on its
// root (css.RootOf), not !important, but for its switching ones, and the marks floe lays out set none; a theme (every
// file, default's palette among them) and floe's base.css set a part's variable on the root of the file of a part or
// look that sets or reads it, or that a part reading it takes (kit's Taken), where it wins, and never a switching one.
// Out of it are floe's own: what base.css and default's palette set on their root, what base.css registers, and
// floesOwn. Themes are files in a directory each, with no directories in them.
func TestPartVariables(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	file := func(name string) []byte {
		src, err := os.ReadFile("../kit/floe/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return src
	}
	partCSS := func(name string) []byte { // a part may have none
		src, err := os.ReadFile("../kit/floe/parts/" + name + "/part.css")
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return src
	}
	ours := func(v string) bool { return floesOwn[v] == "" }

	var amiss []string
	styles, sheet := drawn(t)
	for v, ss := range css.Settings(sheet) {
		for _, s := range ss {
			if strings.HasPrefix(s.Rule, "@layer floe.marks") && ours(v) {
				amiss = append(amiss, "the marks floe lays out set "+v+": a mark's variables are the parts'")
			}
		}
	}
	parts, looks, themes, _ := k.Names()
	names := slices.Concat(parts, looks)
	of := map[string]map[string]bool{} // what each part or look sets or reads
	for _, name := range names {
		src := partCSS(name)
		of[name] = css.Read(src)
		for v, ss := range css.Settings(src) {
			of[name][v] = true
			if !ours(v) {
				continue
			}
			for _, s := range ss {
				if s.Important {
					amiss = append(amiss, name+" sets "+v+" !important on "+s.Rule+", which a theme's value cannot win over: leave !important out")
				}
				if switching[name][v] == "" {
					if fix := partFix(name, s.Rule); fix != "" {
						amiss = append(amiss, name+" sets "+v+" on "+s.Rule+": "+fix)
					}
				}
			}
		}
		for v := range styles[name] {
			of[name][v] = true
			if ours(v) && switching[name][v] == "" {
				amiss = append(amiss, name+" sets "+v+" in an element's style: a style wins over a theme's value; set it on its :scope, "+
					"or, made from the data on each element, list it in switching under "+name+" with its reason")
			}
		}
	}
	// A look's file reaches the parts that take it, its looks' looks too, so what they read is the look's to set; a
	// part's file sets what the looks it takes read, for that part alone.
	reach := map[string]map[string]bool{}
	for _, name := range names {
		reach[name] = maps.Clone(of[name])
	}
	for _, name := range names {
		for _, look := range k.Taken(name) {
			maps.Copy(reach[look], of[name])
			maps.Copy(reach[name], of[look])
		}
	}
	partVar := map[string]bool{}
	for _, vs := range of {
		for v := range vs {
			if ours(v) {
				partVar[v] = true
			}
		}
	}
	files := func(v string) string {
		var at []string
		for _, name := range names {
			if of[name][v] {
				at = append(at, name+".css")
			}
		}
		return strings.Join(at, ", ")
	}
	check := func(at, name string, src []byte) {
		name = strings.TrimSuffix(name, ".css")
		for v, ss := range css.Settings(src) {
			if owner := ownerOf(v); owner != "" {
				amiss = append(amiss, at+" sets "+v+", "+owner+"'s own ("+switching[owner][v]+"): remove it")
				continue
			}
			if !partVar[v] {
				continue
			}
			if !reach[name][v] {
				amiss = append(amiss, at+" sets "+v+", which only the files of the parts and looks that set or read it set: move it to the :scope of "+files(v))
				continue
			}
			for _, s := range ss {
				if fix := themeFix(name, s.Rule); fix != "" {
					amiss = append(amiss, at+" sets "+v+" on "+s.Rule+": "+fix)
				}
			}
		}
	}
	check("base.css", "", file("base.css"))
	for _, theme := range themes {
		dir := "themes/" + theme
		entries, err := os.ReadDir("../kit/floe/" + dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() {
				t.Fatalf("%s/%s: a theme is files, no directory", dir, e.Name())
			}
			if name, ok := strings.CutSuffix(e.Name(), ".css"); ok {
				check(dir+"/"+e.Name(), name, file(dir+"/"+e.Name()))
			}
		}
	}
	slices.Sort(amiss)
	if len(amiss) > 0 {
		t.Error(strings.Join(amiss, "\n"))
	}
}

// atRuleFix says how to bring a declaration out of a rule that takes it off the root.
const atRuleFix = "move it out of that rule: only @media, @supports, @layer and @container keep it on the root"

// nestingFix says how to write unnested a declaration in rule, a rule nested in another, for the part name.
func nestingFix(name, rule string) string {
	preludes := strings.Split(rule, " { ")
	last := strings.TrimSpace(preludes[len(preludes)-1])
	switch {
	case strings.HasPrefix(last, "@"):
		return "write it as " + last + " { :scope { … } }"
	case strings.HasPrefix(last, "&") && css.RootOf(":scope"+strings.TrimPrefix(last, "&"), ":scope") == css.OnTheRoot:
		return "write it unnested, as :scope" + strings.TrimPrefix(last, "&") + " { … }"
	default:
		return "write it unnested, on " + name + "'s :scope"
	}
}

// partFix says how the part name brings a variable set in rule onto its root, "" when it is there.
func partFix(name, rule string) string {
	switch css.RootOf(rule, ":scope") {
	case css.OnTheRoot:
		return ""
	case css.Nested:
		return nestingFix(name, rule)
	case css.AtRule:
		return atRuleFix
	default:
		return "set it on its :scope, which a theme's value on the root then reaches; a variable that switches " +
			"between elements goes in switching, under " + name + ", with its reason"
	}
}

// themeFix says how a theme's file for name moves a variable set in rule to the root, "" when it is there.
func themeFix(name, rule string) string {
	switch css.RootOf(rule, ":scope") {
	case css.OnTheRoot:
		return ""
	case css.Nested:
		return nestingFix(name, rule)
	case css.AtRule:
		return atRuleFix
	default:
		return "move it to " + name + "'s root, :scope (narrowed on that element by a class or :has(), if need be)"
	}
}

// ownerOf returns the part a switching variable is the own of, or "".
func ownerOf(v string) string {
	for part, vs := range switching {
		if vs[v] != "" {
			return part
		}
	}
	return ""
}
