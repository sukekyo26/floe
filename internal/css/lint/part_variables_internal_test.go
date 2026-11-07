package lint

import (
	"bytes"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"

	floecss "github.com/sukekyo26/floe/internal/css"
	"github.com/sukekyo26/floe/internal/kit"
)

// floesOwnWays are the variables of floe's own ways, set by rules of their own and out of this rule, that floe's base.css
// neither sets on its :root nor registers by @property, nor the default palette sets.
var floesOwnWays = map[string]string{
	"--face":           "a face's ground (css.md: a painted face), base.css clears it on a part's box",
	"--on-face":        "a face's text, as --face",
	"--floe-pill-wrap": "a pill's wrapping, base.css clears it on a part's box and tags sets it on its tags",
	"--tone":           "a tone's colour, which floe builds from tone.json (css.md: tone)",
	"--tone-soft":      "a tone's soft colour, as --tone",
	"--tone-fill":      "a tone's fill, as --tone",
}

// partVariables keeps a part's variables where a theme can set them. A part or a look sets a variable on its root
// (rootOf), not !important, but for an exception (a variable that switches between the elements in it, or made from the
// data on each: Key "<part> <variable>"), and the marks floe lays out set none; a theme and floe's base.css set a part's
// variable on the root of the file of a part or look that sets or reads it, or that a part reading it takes, where it
// wins, not !important, and never a part's switching one. Out of it are floe's own: what floe's base.css sets on its
// :root or registers, what the default palette sets, and floesOwnWays.
// LIMIT: a style attribute the pages drawn (every-part) do not hold goes unread; add the data to every-part if a part
// comes to set a variable so.
var partVariables = Rule{Name: "part-variables", Check: func(in *Input) []Finding {
	var found []Finding
	sheets := in.Sheets()
	own := floesOwn(sheets)
	ours := func(v string) bool { return !own[v] }
	switching := map[string][]string{} // a switching variable: the parts it is an exception of
	for _, key := range in.Exceptions() {
		if part, v, ok := strings.Cut(key, " "); ok {
			switching[v] = append(switching[v], part)
		}
	}
	styles, marks, err := drawn(in.Pages())
	if err != nil {
		found = append(found, Finding{"pages", 0, "part-variables", err.Error()})
	}
	for _, m := range marks {
		for v, ds := range declarations(m.src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
			for _, d := range ds {
				if strings.HasPrefix(d.rule, "@layer floe.marks") && ours(v) {
					found = append(found, Finding{m.file, 0, "part-variables", "the marks floe lays out set " + v + ": a mark's variables are the parts'"})
				}
			}
		}
	}
	parts, looks := in.Names()
	names := slices.Concat(parts, looks)
	of := map[string]map[string]bool{} // what each part or look sets or reads
	for _, name := range names {
		of[name] = map[string]bool{}
	}
	for _, s := range sheets {
		if s.Kind != kindPart && s.Kind != kindLook {
			continue
		}
		in.Saw()
		maps.Copy(of[s.Owner], reads(s.Src))
		for v, ds := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
			of[s.Owner][v] = true
			if !ours(v) {
				continue
			}
			for _, d := range ds {
				file, line := s.at(d.at)
				if d.important {
					found = append(found, Finding{file, line, "part-variables", s.Owner + " sets " + v + " !important on " + d.rule +
						", which a theme's value cannot win over: leave !important out"})
				}
				if in.Allowed(s.Owner + " " + v) {
					continue
				}
				if fix := rootFix(d.rule, ":scope", s.Owner, "set it on its :scope, which a theme's value on the root then reaches; "+
					"a variable that switches between elements is an exception of this rule, \""+s.Owner+" "+v+"\", with its reason"); fix != "" {
					found = append(found, Finding{file, line, "part-variables", s.Owner + " sets " + v + " on " + d.rule + ": " + fix})
				}
			}
		}
	}
	for _, page := range slices.Sorted(maps.Keys(styles)) {
		for name, vs := range styles[page] {
			for _, v := range slices.Sorted(maps.Keys(vs)) {
				if of[name] != nil {
					of[name][v] = true
				}
				if ours(v) && !in.Allowed(name+" "+v) {
					found = append(found, Finding{page, 0, "part-variables", name + " sets " + v + " in an element's style: a style wins over a theme's value; " +
						"set it on its :scope, or, made from the data on each element, make it an exception of this rule, \"" + name + " " + v + "\", with its reason"})
				}
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
		for _, look := range in.Taken(name) {
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
				at = append(at, name)
			}
		}
		return strings.Join(at, ", ")
	}
	for _, s := range sheets {
		if s.Kind != kindBase && s.Kind != kindTheme {
			continue
		}
		for v, ds := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
			for _, d := range ds {
				file, line := s.at(d.at)
				var why string
				switch {
				case reachesOwner(in, s.For, names, switching[v]):
					why = "sets " + v + ", " + strings.Join(switching[v], " and ") + "'s own, which switches between its elements: remove it"
				case !partVar[v]:
					continue
				case !reach[s.For][v]:
					why = "sets " + v + ", which only the files of the parts and looks that set or read it set: move it to the :scope of " + files(v)
				case d.important:
					why = "sets " + v + " !important: a theme sets a part's variable as the part does, on its root; leave !important out"
				default:
					fix := rootFix(d.rule, ":scope", s.For, "move it to "+s.For+"'s root, :scope (narrowed on that element by a class or :has(), if need be)")
					if fix == "" {
						continue
					}
					why = "sets " + v + " on " + d.rule + ": " + fix
				}
				found = append(found, Finding{file, line, "part-variables", why})
			}
		}
	}
	return found
}, Allow: []Allow{
	{"chart --c", "the colour of each series (.s1 to .s4)"},
	{"chart --left", "the room for the left axis, from the data (the svg's style)"},
	{"chart --right", "the room for the right axis, from the data (the svg's style)"},
	{"chart --right-narrow", "the room for the right axis on a narrow screen, from the data (the svg's style)"},
	{"code --floe-code-gutter", "the room for the line numbers, with them (.numbered) or without (.lines)"},
	{"code --digits", "the digits of the last line's number, from the data (the style of <code class=\"lines\">)"},
	{"zoom --floe-tool-alpha", "a button's colours, faded while it is disabled"},
}}

// reachesOwner is whether a theme's file for the part or look name (or for none of them: base.css, the palette, the
// markdown, which reach every part) reaches one of the parts owners, whose exception a variable is: the part itself, or a
// look it takes.
func reachesOwner(in *Input, name string, names, owners []string) bool {
	if len(owners) == 0 {
		return false
	}
	if !slices.Contains(names, name) {
		return true
	}
	return slices.ContainsFunc(owners, func(o string) bool { return o == name || slices.Contains(in.Taken(o), name) })
}

// floesOwn returns the variables out of partVariables: floesOwnWays, what floe's base.css sets on its :root or registers
// by @property, and what the default palette sets.
func floesOwn(sheets []Sheet) map[string]bool {
	own := map[string]bool{}
	for v := range floesOwnWays {
		own[v] = true
	}
	for _, s := range sheets {
		switch {
		case s.Kind == kindBase:
			for v, ds := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
				for _, d := range ds {
					if rootOf(d.rule, ":root") == onTheRoot {
						own[v] = true
					}
				}
			}
			toks := tokensOf(s.Src)
			for i, t := range toks {
				if t.Type != css.AtKeywordToken || !strings.EqualFold(string(t.Data), "@property") {
					continue
				}
				for _, n := range toks[i+1:] {
					if !skipped(n) {
						own[string(n.Data)] = true
						break
					}
				}
			}
		case s.Kind == kindTheme && s.Owner == kit.DefaultTheme && s.For == "palette":
			for v := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
				own[v] = true
			}
		}
	}
	return own
}

// sheetOf is the CSS of a page's <style>, and the page's file.
type sheetOf struct {
	file string
	src  []byte
}

// drawn returns, by page and by part, the variables the style attributes in each page set in it, and the CSS of each
// page, read by x/net/html.
func drawn(pages fs.FS) (map[string]map[string]map[string]bool, []sheetOf, error) {
	styles := map[string]map[string]map[string]bool{}
	var sheets []sheetOf
	if pages == nil {
		return styles, nil, nil
	}
	names, err := fs.Glob(pages, "*.html")
	if err != nil {
		return nil, nil, err
	}
	for _, name := range names {
		src, err := fs.ReadFile(pages, name)
		if err != nil {
			return nil, nil, err
		}
		doc, err := html.Parse(bytes.NewReader(floecss.Preprocess(src)))
		if err != nil {
			return nil, nil, err
		}
		file := path.Join("pages", name)
		styles[file] = map[string]map[string]bool{}
		sheet := &bytes.Buffer{}
		var walk func(n *html.Node, part string)
		walk = func(n *html.Node, part string) {
			for _, a := range n.Attr {
				if a.Key == "data-floe-part" {
					part = a.Val
				}
			}
			for _, a := range n.Attr {
				if a.Key == "style" {
					if styles[file][part] == nil {
						styles[file][part] = map[string]bool{}
					}
					for v := range declarations([]byte(a.Val), func(p string) bool { return strings.HasPrefix(p, "--") }) {
						styles[file][part][v] = true
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
		sheets = append(sheets, sheetOf{file, sheet.Bytes()})
	}
	return styles, sheets, nil
}
