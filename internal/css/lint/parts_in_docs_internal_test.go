package lint

import (
	"maps"
	"slices"
	"strings"
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

// partsInDocs keeps the variables a part reads that floe sets, base.css's --floe-* (on a plain :root, and registered by
// @property with their default) and code's --floe-code-* (on a plain :scope), in two directions: each is named, whole, in
// the first column of the table of its heading in parts.md, a variable to a row, with the default that the CSS gives it
// (in the Default column, as code); and each --floe-* that column names is one the CSS sets. A plain root is the rule's
// own, alone in its selector and in no at-rule: a declaration in @media print, or on :scope:has(> .floe-copy), is not the
// default.
// LIMIT: a row of the table that has no --floe-* (the tone's) is not held to the CSS; a variable the table names is held
// to the first heading's CSS that sets it.
var partsInDocs = Rule{Name: "parts-in-docs", Check: func(in *Input) []Finding {
	var found []Finding
	const doc = "agents/parts.md"
	for _, table := range partsDocs {
		set := table.declared(in)
		rows, err := paletteTable(in.Docs(), doc, table.heading)
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
	return found
}}

// baseVariables returns the --floe-* that floe's base.css sets on a plain :root or registers by @property, with the value
// a part reads: the last one written on the root, else the @property's initial-value.
func baseVariables(in *Input) map[string]string {
	out := map[string]string{}
	for _, s := range in.Sheets() {
		if s.Kind != kindBase {
			continue
		}
		maps.Copy(out, propertyInitials(s.Src, "--floe-"))
		maps.Copy(out, plainDeclared(s.Src, ":root", "--floe-"))
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
			at, name, ok := strings.Cut(strings.TrimSpace(d.rule), " ")
			if ok && strings.EqualFold(at, "@property") && strings.HasPrefix(name, prefix) && len(d.value) > 0 {
				out[name] = valueOf(src, d)
			}
		}
	}
	return out
}

// valueOf returns the value of the declaration d of src, its whitespace collapsed to single spaces.
func valueOf(src []byte, d decl) string {
	last := d.value[len(d.value)-1]
	return strings.Join(strings.Fields(string(src[d.value[0].At:last.At+len(last.Data)])), " ")
}
