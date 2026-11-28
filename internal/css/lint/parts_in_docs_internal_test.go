package lint

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// held is a default floe's CSS gives a variable: the file that writes it, the part or look the file is of ("" for base.css
// and defaults.css), whether the file reads it with that default (var(--x, default)) rather than sets it, and the value,
// its whitespace collapsed.
type held struct {
	file, owner string
	read        bool
	value       string
}

// varDocs are the tables of the docs held to floe's CSS: the doc and the heading over each, the defaults of the CSS it is
// held to, the names of its first column held (those that start with names), what it says of a name the CSS gives no
// default (nowhere), and the variables of the CSS the table must name (missing: what is amiss of those it does not name,
// nil for a table that names some alone).
var varDocs = []struct {
	doc, heading string
	values       func(in *Input) map[string][]held
	names        string
	nowhere      string
	missing      func(in *Input, values map[string][]held, named map[string]bool) []string
}{
	{"agents/parts.md", "Variables floe sets", baseVariables, "--floe-", "floe's CSS sets nowhere", unnamed},
	{"agents/parts.md", "Code's look", codeVariables, "--floe-", "floe's CSS sets nowhere", unnamed},
	{"agents/themes.md", "floe's own variables", baseVariables, "--floe-", "floe's CSS sets nowhere", nil},
	{"agents/themes.md", "Padding and text size", baseVariables, "--floe-", "floe's CSS sets nowhere", nil},
	{
		"agents/themes.md", "Part variables", partDefaults, "--",
		"floe's parts and looks neither set on their :scope nor read with a default, nor base.css reads with one", partsUnnamed,
	},
}

// byPart are the variables a table writes part by part, as part `value`, whether or not the parts' values are alike.
var byPart = []string{"--floe-mark", "--floe-mark-gap"}

// partsInDocs keeps the tables of the variables in the agents' docs (varDocs) to the defaults floe's CSS gives them: each
// name a table holds is one the CSS gives a default, and the row's Default column (code) writes that default, each file's
// alike, part by part for byPart; a table of a missing names every variable it says. Part variables names every
// variable a part or a look of floe's sets on its :scope or reads with a default, but floe's own (floesOwn) and the rule's
// exceptions, which a theme does not set; an exception named in a table (Code's look names code's) is held all the same.
// LIMIT: a value on the root in an at-rule (@media print { :scope { --x: … } }) is no default and is not held; read it if
// a part comes to have a theme set a value it puts on its root in an at-rule.
// LIMIT: a value on a root narrowed otherwise than by classes (:scope:has(…), :scope[open]) is a state, no default, and is
// not held; read it if a table comes to write a value of a state.
// LIMIT: what themes and templates read with a default is not read (report's and tech-spec's --floe-heading-*, in themes.md's
// Section headings, alone); read it if one comes to read a part's variable so.
// LIMIT: a row may name several variables, its Default giving each its own in order: a row of two variables of one
// description passes, `| --floe-pad-block, --floe-pad-inline | … | A box's padding above and below its text | 1.1rem,
// 1.3rem |`, where parts.md writes a variable to a row. Refuse it in parts.md again if such a row comes.
// LIMIT: the tables of "Rows with a mark" (no Default column) and "Variables a part sets" (floe's own) are not held, nor
// the Set in column of any.
// A second check holds the classes of the section "Code's look": each `.class` that a code span of it names alone is one
// that a selector of code's part.css, outside a comment, has.
var partsInDocs = Rule{Name: "parts-in-docs", Allow: []Allow{
	{"--floe-code-pad", "code's, which the parts of code's look read; a theme does not set it again (css.md code-lines), and parts.md's Code's look names it"},
	{"--floe-code-room", "code's, as --floe-code-pad"},
	{"--floe-code-faint", "code's, as --floe-code-pad"},
	{"--floe-code-head-faint", "code's, as --floe-code-pad"},
	{"--tabs-face-bg", "tabs makes it on its root from the face's two colours, and it wins over --tabs-picked-* in a face (themes.md's tabs)"},
	{"--tabs-face-fg", "tabs makes it on its root, as --tabs-face-bg"},
	{"--tabs-title", "tabs makes it on its root from a face's text colour"},
	{"--tabs-ring", "tabs makes it on its root from a face's text colour"},
	{"--tabs-print", "tabs makes it on its root from a face's text colour"},
	{"--links-title", "links makes it on its root from a face's text colour, as tabs' --tabs-title"},
	{"--links-to", "links makes it on its root from a face's text colour, as --links-title"},
	{"--links-words", "links makes it on its root from a face's text colour, as --links-title"},
	{"--links-line", "links makes it on its root from a face's text colour, as --links-title"},
	{"--tabs-gap", "the gap between tabs' titles, which the panel's margin takes off --floe-room (tabs/part.css's LIMIT)"},
}, Check: func(in *Input) []Finding {
	var found []Finding
	trees := map[string]*html.Node{}
	unread := map[string]bool{} // a doc that cannot be read, told once
	for _, table := range varDocs {
		if unread[table.doc] {
			continue
		}
		root, ok := trees[table.doc]
		if !ok {
			var err string
			if root, err = docTree(in.Docs(), table.doc); err != "" {
				unread[table.doc] = true
				found = append(found, Finding{table.doc, 0, "parts-in-docs", err})
				continue
			}
			trees[table.doc] = root
		}
		rows, err := paletteTable(root, table.heading, table.heading)
		if err != "" {
			found = append(found, Finding{table.doc, 0, "parts-in-docs", err})
			continue
		}
		values := table.values(in)
		say := func(text string) {
			found = append(found, Finding{table.doc, 0, "parts-in-docs", table.heading + ": " + text})
		}
		named := map[string]bool{}
		for _, row := range rows {
			keep := func(name string) bool {
				_, ok := values[name]
				return strings.HasPrefix(name, table.names) && ok
			}
			for _, name := range row.names {
				if !strings.HasPrefix(name, table.names) {
					continue
				}
				named[name] = true
				if _, ok := values[name]; !ok {
					say("names " + name + ", which " + table.nowhere + ": remove its row, or set it")
				} else {
					in.Saw()
				}
			}
			for _, text := range rowDefaults(row, values, keep) {
				say(text)
			}
		}
		if table.missing != nil {
			for _, text := range table.missing(in, values, named) {
				say(text)
			}
		}
	}
	if root, ok := trees["agents/parts.md"]; ok {
		found = append(found, codeClasses(in, root, "agents/parts.md")...)
	}
	return found
}}

// rowDefaults returns what is amiss of the Default column of row against values, for each name of the row keep takes: a
// variable of byPart by its part `value` pairs, any order; another by the code of its place in the row, which every
// file's default of it is. A default that differs is told once for each file that writes it.
func rowDefaults(row paletteRow, values map[string][]held, keep func(name string) bool) []string {
	var kept []string
	for _, name := range row.names {
		if keep(name) {
			kept = append(kept, name)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	var out []string
	if !row.hasDefault || len(row.defaults) == 0 {
		for _, name := range kept {
			h := values[name][0]
			if slices.Contains(byPart, name) {
				out = append(out, "gives "+name+" no default: write each part's default after its name, as "+partOf(h)+" `"+h.value+"`, in its Default column")
			} else {
				out = append(out, "gives "+name+" no default: write `"+h.value+"` in code, in its Default column")
			}
		}
		return out
	}
	if i := slices.IndexFunc(kept, func(n string) bool { return slices.Contains(byPart, n) }); i >= 0 {
		if len(row.names) > 1 {
			other := row.names[0]
			if other == kept[i] {
				other = row.names[1]
			}
			return []string{"the row of " + kept[i] + " names " + other + " too: write a variable written part by part in a row of its own"}
		}
		return partDefaultsAmiss(kept[i], row, values[kept[i]])
	}
	if len(row.defaults) != len(row.names) {
		return []string{"the row of " + row.names[0] + " names " + strconv.Itoa(len(row.names)) + " variables and gives " + strconv.Itoa(len(row.defaults)) +
			" defaults: give each its default in code, in order"}
	}
	for i, name := range row.names {
		if !keep(name) {
			continue
		}
		want := row.defaults[i]
		told := map[string]bool{}
		for _, h := range values[name] {
			if h.value == want || told[h.file+"\x00"+h.value] {
				continue
			}
			told[h.file+"\x00"+h.value] = true
			text := "gives " + name + " the default " + want + ", which " + h.file + " writes as " + h.value
			if slices.ContainsFunc(values[name], func(o held) bool { return o.value == want && o.file != h.file }) {
				text += ": give it one default, or, if it differs part by part, add it to the rule's variables written part by part"
			}
			out = append(out, text)
		}
	}
	return out
}

// partDefaultsAmiss returns what is amiss of the row of name, a variable of byPart, against its defaults, by its part
// `value` pairs: the part of a default is the file's part or look, or the file for base.css.
func partDefaultsAmiss(name string, row paletteRow, defaults []held) []string {
	type pair struct{ part, value string }
	written := map[pair]bool{}
	firstOf := map[string]string{} // a part's first value in the row
	for i, value := range row.defaults {
		written[pair{row.labels[i], value}] = true
		if _, ok := firstOf[row.labels[i]]; !ok {
			firstOf[row.labels[i]] = value
		}
	}
	actual := map[pair]bool{}
	var out []string
	told := map[string]bool{}
	for _, h := range defaults {
		p := pair{partOf(h), h.value}
		if actual[p] {
			continue // a part that sets it and reads it with the same default
		}
		actual[p] = true
		switch first, ok := firstOf[p.part]; {
		case written[p]:
		case ok:
			told[p.part] = true
			out = append(out, "gives "+name+" the default "+first+" for "+p.part+", which "+h.file+" writes as "+h.value)
		default:
			out = append(out, "gives "+name+" no default for "+p.part+", which "+h.file+" writes as "+h.value+
				": write "+p.part+" `"+h.value+"` in its Default column")
		}
	}
	for i, value := range row.defaults {
		part := row.labels[i]
		switch {
		case part == "":
			out = append(out, "gives "+name+" the default "+value+" with no part before it: write each default after the name of the part that sets it")
		case !slices.ContainsFunc(defaults, func(h held) bool { return partOf(h) == part }):
			out = append(out, "gives "+name+" the default "+value+" for "+part+", which sets none: write each default after the name of the part that sets it, or remove it")
		case !actual[pair{part, value}] && !told[part]:
			told[part] = true
			h := defaults[slices.IndexFunc(defaults, func(h held) bool { return partOf(h) == part })]
			out = append(out, "gives "+name+" the default "+value+" for "+part+", which "+h.file+" writes as "+h.value)
		}
	}
	return out
}

// partOf returns the part of a default: its file's part or look, or the file for base.css.
func partOf(h held) string {
	if h.owner == "" {
		return h.file
	}
	return h.owner
}

// unnamed returns what is amiss of a table that names every variable of values: each it does not name.
func unnamed(_ *Input, values map[string][]held, named map[string]bool) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if !named[name] {
			out = append(out, "names no "+name+", which floe's CSS sets: write `"+name+"` in its table")
		}
	}
	return out
}

// partsUnnamed returns what is amiss of Part variables: each variable a part or a look sets on its :scope or reads with a
// default that the table does not name, but floe's own and the rule's exceptions.
func partsUnnamed(in *Input, values map[string][]held, named map[string]bool) []string {
	own := floesOwn(in.Sheets())
	var out []string
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if named[name] || own[name] {
			continue
		}
		i := slices.IndexFunc(values[name], func(h held) bool { return h.owner != "" })
		if i < 0 || in.Allowed(name) {
			continue
		}
		h := values[name][i]
		how := "sets on its :scope"
		if h.read {
			how = "reads with a default"
		}
		out = append(out, "names no "+name+", which "+h.file+" "+how+": write a row for it, or allow it in the rule with why a theme does not set it")
	}
	return out
}

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

// baseVariables returns the --floe-* that floe's defaults.css sets on its :root and base.css registers by @property,
// with the @property's initial-value.
func baseVariables(in *Input) map[string][]held {
	out := map[string][]held{}
	for _, s := range in.Sheets() {
		switch s.Kind {
		case kindBase:
			for name, value := range propertyInitials(s.Src, "--floe-") {
				out[name] = append(out[name], held{file: s.File, value: value})
			}
		case kindDefaults:
			addRoot(out, s, ":root", "--floe-")
		}
	}
	return out
}

// codeVariables returns the --floe-code-* that the part code's part.css sets on its :scope.
func codeVariables(in *Input) map[string][]held {
	out := map[string][]held{}
	for _, s := range in.Sheets() {
		if s.Kind == kindPart && s.Owner == "code" {
			addRoot(out, s, ":scope", "--floe-code-")
		}
	}
	return out
}

// partDefaults returns the variables each part and look of floe's sets on its :scope or reads with a default, and those
// base.css reads with one, with each default, in the order of the sheets.
func partDefaults(in *Input) map[string][]held {
	out := map[string][]held{}
	for _, s := range in.Sheets() {
		if s.Kind != kindPart && s.Kind != kindLook && s.Kind != kindBase {
			continue
		}
		if s.Kind != kindBase {
			addRoot(out, s, ":scope", "--")
		}
		for _, r := range varReads(s.Src) {
			if r.def != "" {
				out[r.name] = append(out[r.name], held{file: s.File, owner: s.Owner, read: true, value: r.def})
			}
		}
	}
	return out
}

// addRoot adds to out the variables of s that start with prefix, set on its root (rootValues).
func addRoot(out map[string][]held, s Sheet, root, prefix string) {
	values := rootValues(s.Src, root, func(p string) bool { return strings.HasPrefix(p, prefix) })
	for _, name := range slices.Sorted(maps.Keys(values)) {
		for _, value := range values[name] {
			out[name] = append(out[name], held{file: s.File, owner: s.Owner, value: value})
		}
	}
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
