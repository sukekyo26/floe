package kit

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// schemaDef is the part of a JSON Schema definition of the diagram part that the doc's tables of keys are held to.
type schemaDef struct {
	Ref         string                `json:"$ref"`
	Enum        []string              `json:"enum"`
	Required    []string              `json:"required"`
	Properties  map[string]*schemaDef `json:"properties"`
	Items       *schemaDef            `json:"items"`
	PrefixItems []*schemaDef          `json:"prefixItems"`
}

// docKeyRow is a row of a table of keys in docs/agents/diagrams.md: the keys of its first column (the code in it), and
// its second column, what the key takes.
type docKeyRow struct {
	keys  []string
	takes string
	code  []string
}

// docTables returns the rows of the first table under each heading (h2 to h4) of docs/agents/<file>, by the heading's text.
func docTables(t *testing.T, file string) map[string][]docKeyRow {
	t.Helper()

	tables := map[string][]docKeyRow{}
	heading := ""
	for n := range agentDoc(t, file).Descendants() {
		switch {
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3 || n.DataAtom == atom.H4:
			heading = textOf(n)
		case n.DataAtom == atom.Table && tables[heading] == nil:
			for tr := range n.Descendants() {
				if tr.DataAtom != atom.Tr || tr.Parent.DataAtom != atom.Tbody {
					continue
				}
				var cells []*html.Node
				for td := tr.FirstChild; td != nil; td = td.NextSibling {
					if td.DataAtom == atom.Td {
						cells = append(cells, td)
					}
				}
				if len(cells) < 2 {
					continue
				}
				row := docKeyRow{takes: textOf(cells[1])}
				for c := range cells[0].Descendants() {
					if c.DataAtom == atom.Code {
						row.keys = append(row.keys, textOf(c))
					}
				}
				for c := range cells[1].Descendants() {
					if c.DataAtom == atom.Code {
						row.code = append(row.code, textOf(c))
					}
				}
				tables[heading] = append(tables[heading], row)
			}
		}
	}
	return tables
}

// docCodes returns the code under the heading (h2 to h4) of docs/agents/<file>, up to the next heading.
func docCodes(t *testing.T, file, heading string) []string {
	t.Helper()

	var codes []string
	under := false
	for n := range agentDoc(t, file).Descendants() {
		switch {
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3 || n.DataAtom == atom.H4:
			under = textOf(n) == heading
		case under && n.DataAtom == atom.Code:
			codes = append(codes, textOf(n))
		}
	}
	return codes
}

// deref follows the `$ref`s of def to the definition they come to, JSON pointers into the schema included
// (`#/$defs/box/properties/shape`). A `$ref` it cannot follow fails the test, so that none is skipped; only tone.json,
// which TestDiagramsDocTones holds, comes to nil.
func deref(t *testing.T, def *schemaDef, doc any) *schemaDef {
	t.Helper()

	for range 8 {
		switch {
		case def.Ref == "":
			return def
		case def.Ref == "floe:///parts/tone.json":
			return nil
		case !strings.HasPrefix(def.Ref, "#/"):
			t.Errorf("the schema's $ref %q is one TestDiagramsDocKeys does not follow: teach it to", def.Ref)
			return nil
		}
		node := doc
		for _, seg := range strings.Split(strings.TrimPrefix(def.Ref, "#/"), "/") {
			seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
			m, ok := node.(map[string]any)
			if !ok || m[seg] == nil {
				t.Errorf("the schema's $ref %q points at nothing", def.Ref)
				return nil
			}
			node = m[seg]
		}
		raw, err := json.Marshal(node)
		if err != nil {
			t.Fatal(err)
		}
		def = &schemaDef{}
		if err := json.Unmarshal(raw, def); err != nil {
			t.Fatal(err)
		}
	}
	t.Error("a chain of $refs in the schema is longer than TestDiagramsDocKeys follows")
	return nil
}

// The tables of keys in docs/agents/diagrams.md, which floe diagram guide prints, name for each part (and for the spec,
// the nodes of a tree and the nodes and edges of a graph) the keys its schema takes and no other, mark a key as required
// when the schema requires it, and write the values of a key of an enum as the schema lists them. The parts, and the
// nodes of a graph, are read from the schema's lists of them, so that a part the schema gains and the doc leaves out
// fails here.
// LIMIT: the values of a tone are not held here: tone.json's are checked on the sentence that lists them. Nor are the
// keys that a schema requires by `if`/`then`/`else` (a graph's `nodes` or `lanes`, a label's `x` and `y`), which are not
// in `required`: their rows say so in words, and a row that wrongly says "required" fails. Nor are the values inside a
// list (`items`, `prefixItems`), but the styles of a box's row, which a test below holds.
func TestDiagramsDocKeys(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("floe/parts/diagram/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]*schemaDef `json:"$defs"`
		Then schemaDef             `json:"then"`
		Else schemaDef             `json:"else"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	root := &schemaDef{Required: schema.Else.Required, Properties: map[string]*schemaDef{}}
	maps.Copy(root.Properties, schema.Else.Properties)
	maps.Copy(root.Properties, schema.Then.Properties)

	// The heading of each table, and the definition it is held to.
	held := map[string]*schemaDef{"The spec": root}
	hold := func(heading string, ref *schemaDef) {
		if ref == nil {
			t.Fatalf("the schema has no definition for %q", heading)
		}
		held[heading] = deref(t, ref, doc)
	}
	parts := schema.Else.Properties["parts"]
	if parts == nil || parts.Items == nil {
		t.Fatal("the schema lists no parts")
	}
	for kind, ref := range parts.Items.Properties {
		hold(kind, ref)
	}
	nodes := schema.Defs["graphNodes"]
	if nodes == nil || nodes.Items == nil {
		t.Fatal("the schema lists no nodes of a graph")
	}
	for kind, ref := range nodes.Items.Properties {
		article := "A"
		if strings.ContainsRune("aeiou", rune(kind[0])) {
			article = "An"
		}
		hold(article+" "+kind+" of a graph", ref)
	}
	hold("An edge of a graph", schema.Defs["graph"].Properties["edges"].Items)
	hold("A node of a tree", schema.Defs["tree"].Properties["root"])

	tables := docTables(t, "diagrams.md")
	for _, heading := range slices.Sorted(maps.Keys(held)) {
		def, rows := held[heading], tables[heading]
		if def == nil || rows == nil {
			t.Errorf("docs/agents/diagrams.md has no table of keys under %q, or the schema no definition for it", heading)
			continue
		}
		var got []string
		for _, row := range rows {
			for _, key := range row.keys {
				got = append(got, key)
				prop, ok := def.Properties[key]
				if !ok {
					continue
				}
				if want := slices.Contains(def.Required, key); want != strings.Contains(row.takes, "required") {
					t.Errorf("docs/agents/diagrams.md, %q: %s is required in the schema: %t, and the table says %q", heading, key, want, row.takes)
				}
				var enum []string
				if end := deref(t, prop, doc); end != nil {
					enum = end.Enum
				}
				if enum != nil && !slices.Equal(slices.Sorted(slices.Values(row.code)), slices.Sorted(slices.Values(enum))) {
					t.Errorf("docs/agents/diagrams.md, %q: %s takes %v in the schema, and the table writes %v", heading, key, enum, row.code)
				}
			}
		}
		want := slices.Sorted(maps.Keys(def.Properties))
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("docs/agents/diagrams.md, %q: the table names the keys %v, the schema takes %v: write a row for each, and no other", heading, got, want)
		}
	}

	// The styles of a row of a box are the second value of its `[text, style]`.
	row := schema.Defs["box"].Properties["rows"].Items
	if row == nil || len(row.PrefixItems) != 2 || row.PrefixItems[1].Enum == nil {
		t.Fatal("the rows of a box are no longer [text, style] of an enum of styles: teach TestDiagramsDocKeys the new form")
	}
	written := docCodes(t, "diagrams.md", "box")
	for _, style := range row.PrefixItems[1].Enum {
		if !slices.Contains(written, style) {
			t.Errorf("docs/agents/diagrams.md, \"box\": the style %q of a row is not written in code", style)
		}
	}
}

// The sentence of docs/agents/diagrams.md that lists the tones lists those of tone.json, which the schema of every part
// that takes a tone names.
func TestDiagramsDocTones(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("floe/parts/tone.json")
	if err != nil {
		t.Fatal(err)
	}
	var tone schemaDef
	if err := json.Unmarshal(raw, &tone); err != nil {
		t.Fatal(err)
	}
	for p := range agentDoc(t, "diagrams.md").Descendants() {
		if p.DataAtom != atom.P || !strings.HasPrefix(textOf(p), "tone is one of") {
			continue
		}
		var got []string
		for c := range p.Descendants() {
			if c.DataAtom == atom.Code {
				got = append(got, textOf(c))
			}
		}
		if len(got) == 0 || got[0] != "tone" || !slices.Equal(slices.Sorted(slices.Values(got[1:])), slices.Sorted(slices.Values(tone.Enum))) {
			t.Errorf("docs/agents/diagrams.md lists the tones %v, tone.json has %v", got, tone.Enum)
		}
		return
	}
	t.Error("docs/agents/diagrams.md has no sentence that begins \"tone is one of\": list the tones of tone.json there")
}
