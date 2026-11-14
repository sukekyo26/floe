package lint

import (
	"maps"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// partsDocs are the tables of docs/agents/parts.md held to floe's CSS: the heading over each, and the variables of the
// CSS that the table names.
var partsDocs = []struct {
	heading string
	// declared returns the variables, by name, that floe's CSS sets, with their values.
	declared func(in *Input) map[string]string
}{
	{"Variables floe sets", baseVariables},
	{"Code's look", codeVariables},
}

// partsInDocs keeps the variables a part reads that floe sets, defaults.css's --floe-* (on a plain :root) and base.css's
// (registered by @property with their default) and code's --floe-code-* (on a plain :scope), in two directions: each is named, whole, in
// the first column of the table of its heading in parts.md, a variable to a row, with the default that the CSS gives it
// (in the Default column, as code); and each --floe-* that column names is one the CSS sets. A plain root is the rule's
// own, alone in its selector and in no at-rule: a declaration in @media print, or on :scope:has(> .floe-copy), is not the
// default.
// LIMIT: a row of the table that has no --floe-* (the tone's) is not held to the CSS; each table is held to the CSS of its
// own heading alone (partsDocs): the first to defaults.css's and base.css's, the second to code's. The tables of "Rows with a mark" (--floe-mark,
// --floe-mark-gap) and "Variables a part sets" (--floe-pill-wrap) are not held to the CSS either: a part sets those, and the
// default differs part by part.
// A third check holds the classes of the section "Code's look": each `.class` that a code span of it names alone is one
// that a selector of code's part.css, outside a comment, has.
var partsInDocs = Rule{Name: "parts-in-docs", Check: func(in *Input) []Finding {
	const doc = "agents/parts.md"
	root, err := docTree(in.Docs(), doc)
	if err != "" {
		return []Finding{{doc, 0, "parts-in-docs", err}}
	}
	var found []Finding
	for _, table := range partsDocs {
		set := table.declared(in)
		rows, err := paletteTable(root, table.heading, table.heading)
		if err != "" {
			found = append(found, Finding{doc, 0, "parts-in-docs", err})
			continue
		}
		named := map[string]bool{}
		for _, row := range rows {
			if len(row.names) > 1 {
				found = append(found, Finding{doc, 0, "parts-in-docs", "the row of " + row.names[0] + " names " + row.names[1] + " too: write a variable to a row, in " + table.heading})
			}
			for i, name := range row.names {
				if !strings.HasPrefix(name, "--floe-") {
					continue
				}
				named[name] = true
				want, ok := set[name]
				if !ok {
					found = append(found, Finding{doc, 0, "parts-in-docs", "names " + name + ", which floe's CSS sets nowhere: remove its row from " + table.heading + ", or set it"})
					continue
				}
				in.Saw()
				switch {
				case !row.hasDefault || i >= len(row.defaults):
					found = append(found, Finding{doc, 0, "parts-in-docs", "gives " + name + " no default: write `" + want + "` in code, in its Default column"})
				case row.defaults[i] != want:
					found = append(found, Finding{doc, 0, "parts-in-docs", "gives " + name + " the default " + row.defaults[i] + ", which floe's CSS writes as " + want})
				}
			}
		}
		for _, name := range slices.Sorted(maps.Keys(set)) {
			if !named[name] {
				found = append(found, Finding{doc, 0, "parts-in-docs", "names no " + name + ", which floe's CSS sets: write `" + name + "` in the table of " + table.heading})
			}
		}
	}
	return append(found, codeClasses(in, root, doc)...)
}}

// codeClasses returns a finding for each class that a code span of the section "Code's look" of the doc names alone and
// no selector of code's part.css has: such a part that writes the class in its HTML is styled by the look.
func codeClasses(in *Input, root *html.Node, doc string) []Finding {
	have := map[string]bool{}
	for _, s := range in.Sheets() {
		if s.Kind == kindPart && s.Owner == "code" {
			for _, class := range selectorClasses(tokensOf(s.Src)) {
				have[class] = true
			}
		}
	}
	var found []Finding
	under := false
	for n := range root.Descendants() {
		switch {
		case n.DataAtom == atom.H2:
			under = textOf(n) == "Code's look"
		case under && n.DataAtom == atom.Code:
			span := textOf(n)
			if class := classesOf(tokensOf([]byte(span))); len(class) == 1 && class[0] == span {
				in.Saw()
				if !have[span] {
					found = append(found, Finding{doc, 0, "parts-in-docs", "names the class " + span + " in Code's look, which a selector of code's part.css does not have: write the classes it does"})
				}
			}
		}
	}
	return found
}

// selectorClasses returns the classes of the selectors of toks: those in the tokens before a rule's {, from the last
// rule token (a {, a ; or a }) before it, as rules marks them. A rule of no declaration has its selectors too.
func selectorClasses(toks []token) []string {
	_, _, ruling := rules(toks)
	var out []string
	from := 0
	for i, t := range toks {
		if !ruling[i] {
			continue
		}
		if t.Type == css.LeftBraceToken {
			out = append(out, classesOf(toks[from:i])...)
		}
		from = i + 1
	}
	return out
}

// classesOf returns the classes (".name") that toks write: a delimiter . and the identifier right after it.
func classesOf(toks []token) []string {
	var out []string
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].Type == css.DelimToken && string(toks[i].Data) == "." && toks[i+1].Type == css.IdentToken {
			out = append(out, "."+string(toks[i+1].Data))
		}
	}
	return out
}

// baseVariables returns the --floe-* that floe's defaults.css sets on a plain :root and base.css registers by @property,
// with the value a part reads: the last one written on the root, else the @property's initial-value.
func baseVariables(in *Input) map[string]string {
	out := map[string]string{}
	for _, s := range in.Sheets() {
		switch s.Kind {
		case kindBase:
			maps.Copy(out, propertyInitials(s.Src, "--floe-"))
		case kindDefaults:
			maps.Copy(out, plainDeclared(s.Src, ":root", "--floe-"))
		}
	}
	return out
}

// codeVariables returns the --floe-code-* that the part code's part.css sets on a plain :scope.
func codeVariables(in *Input) map[string]string {
	out := map[string]string{}
	for _, s := range in.Sheets() {
		if s.Kind == kindPart && s.Owner == "code" {
			maps.Copy(out, plainDeclared(s.Src, ":scope", "--floe-code-"))
		}
	}
	return out
}

// plainDeclared returns the variables of src that start with prefix, declared in a rule whose selector is root alone (and
// in no rule around it), with the value of the last such declaration.
func plainDeclared(src []byte, root, prefix string) map[string]string {
	out := map[string]string{}
	for name, ds := range declarations(src, func(p string) bool { return strings.HasPrefix(p, prefix) }) {
		for _, d := range ds {
			if strings.TrimSpace(d.rule) == root && len(d.value) > 0 {
				out[name] = valueOf(src, d)
			}
		}
	}
	return out
}

// propertyInitials returns the variables that start with prefix and an @property registers, with its initial-value.
func propertyInitials(src []byte, prefix string) map[string]string {
	out := map[string]string{}
	for _, ds := range declarations(src, func(p string) bool { return p == "initial-value" }) {
		for _, d := range ds {
			if at := strings.Fields(d.rule); len(at) == 2 && strings.EqualFold(at[0], "@property") && strings.HasPrefix(at[1], prefix) && len(d.value) > 0 {
				out[at[1]] = valueOf(src, d)
			}
		}
	}
	return out
}

// valueOf returns the value of the declaration d of src, its whitespace collapsed to single spaces.
func valueOf(src []byte, d decl) string {
	return strings.Join(strings.Fields(text(src, d.value)), " ")
}
