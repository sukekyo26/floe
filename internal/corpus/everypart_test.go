package corpus_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/cli"
)

// everyPart is the sample of every part, which the browser's test of the colours draws in every theme: what it
// leaves out is not measured.
const everyPart = "testdata/every-part"

// floeParts are floe's parts, as kit embeds them.
const floeParts = "../kit/floe/parts"

// tones are the classes a part draws a tone in, and "" for one left out.
var tones = []string{"accent", "good", "warn", "bad", ""}

// The sample of every part draws every part of floe's but a look (one of no schema), one of no style and the
// diagram, each
// part that takes a tone in every tone and in none (an element of the same kind as one drawn in a tone, without
// one), and every value of each enum of a part's schema (findings' level, steps' state) in its data.
func TestEveryPartDrawsEveryPart(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile(filepath.Join(everyPart, "every-part.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Drawn from the data now, not read from golden/: a value left out fails here before golden/ is written anew.
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	env := cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: everyPart, Getenv: func(string) string { return "" }}
	if code := cli.Run([]string{"render", "--out-dir", out, "every-part.yaml"}, env); code != 0 {
		t.Fatalf("floe render = %d: %s", code, stderr.String())
	}
	page, err := os.ReadFile(filepath.Join(out, "every-part.html"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		t.Fatal(err)
	}
	drawn := map[string]*drawnTones{} // by the part's name
	for n := range doc.Descendants() {
		if name := nodeAttr(n, "data-floe-part"); name != "" {
			if drawn[name] == nil {
				drawn[name] = &drawnTones{found: map[string]bool{}, toned: map[string]bool{}, plain: map[string]bool{}}
			}
			drawn[name].read(n)
		}
	}
	var data any
	if err = yaml.Unmarshal(src, &data); err != nil {
		t.Fatal(err)
	}
	parts := readParts(t)
	written := map[string]map[string]bool{} // path=value of each part's data, by the part's name
	readOccurrences(data, parts, written)
	for _, name := range slices.Sorted(maps.Keys(parts)) {
		p := parts[name]
		if p.schema == nil || p.css == "" || name == "diagram" {
			// A look draws nothing of its own, and a part of no style (script) no colour. The diagram has samples of
			// its own (diagram/, examples/), and its text, in SVG, is not measured.
			continue
		}
		if drawn[name] == nil {
			t.Errorf("every-part draws no %s", name)
			continue
		}
		if p.takesTone(p.schema, true, map[string]bool{}) {
			for _, tone := range tones {
				if !drawn[name].has(tone) {
					t.Errorf("every-part draws %s in no tone %q", name, tone)
				}
			}
		}
		enums := map[string][]string{}
		p.enumsOf(p.schema, "", enums, true, map[string]bool{})
		for _, path := range slices.Sorted(maps.Keys(enums)) {
			for _, v := range enums[path] {
				if !written[name][path+"="+v] {
					t.Errorf("every-part draws %s with no %s: %s", name, strings.TrimPrefix(path, "."), v)
				}
			}
		}
	}
}

// part is one of floe's parts: its schema (nil for a look) and its style.
type part struct {
	schema map[string]any
	css    string
}

// readParts reads floe's parts by their names.
func readParts(t *testing.T) map[string]*part {
	t.Helper()

	dirs, err := os.ReadDir(floeParts)
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]*part{}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		p := &part{schema: nil, css: ""}
		if b, err := os.ReadFile(filepath.Join(floeParts, d.Name(), "schema.json")); err == nil {
			if err := json.Unmarshal(b, &p.schema); err != nil {
				t.Fatal(err)
			}
		}
		if b, err := os.ReadFile(filepath.Join(floeParts, d.Name(), "part.css")); err == nil {
			p.css = strings.TrimSpace(string(b))
		}
		parts[d.Name()] = p
	}
	return parts
}

// toneRef is the schema of a tone, which a part draws in a class of each value.
const toneRef = "floe:///parts/tone.json"

// held is a schema another holds: key is the property it is the schema of, "" for one in the same place (an
// item, one of $defs referred to, a branch of anyOf, oneOf, allOf or if).
type held struct {
	key    string
	schema map[string]any
}

// sub returns the schemas s holds, the ones of $defs it refers to among them, and whether s is another part's.
// A schema of $defs is held once a walk, as seen keeps them: one that refers to itself (a tree) ends the walk.
func (p *part) sub(s map[string]any, seen map[string]bool) ([]held, bool) {
	var hs []held
	add := func(key string, v any) {
		if m, ok := v.(map[string]any); ok {
			hs = append(hs, held{key: key, schema: m})
		}
	}
	if ref, ok := s["$ref"].(string); ok {
		name, local := strings.CutPrefix(ref, "#/$defs/")
		if !local {
			return nil, true
		}
		if !seen[name] {
			seen[name] = true
			defs, _ := p.schema["$defs"].(map[string]any) //nolint:errcheck // none is none
			add("", defs[name])
		}
	}
	if props, ok := s["properties"].(map[string]any); ok {
		for k, v := range props {
			add(k, v)
		}
	}
	for _, key := range []string{"items", "additionalProperties", "if", "then", "else"} {
		add("", s[key])
	}
	for _, key := range []string{"prefixItems", "anyOf", "oneOf", "allOf"} {
		if list, ok := s[key].([]any); ok {
			for _, v := range list {
				add("", v)
			}
		}
	}
	return hs, false
}

// takesTone reports whether s, a schema of p, takes a tone, root, but not through a part it holds.
func (p *part) takesTone(s map[string]any, root bool, seen map[string]bool) bool {
	if s["$ref"] == toneRef {
		return true
	}
	hs, other := p.sub(s, seen)
	if other && !root {
		return false
	}
	for _, h := range hs {
		if p.takesTone(h.schema, false, seen) {
			return true
		}
	}
	return false
}

// enumsOf adds to enums the values of each enum under s, a schema of p at path, by its path as valuesIn writes
// it; a part's own, but not those of the parts it holds, nor the tones, which a part draws in a class of each.
func (p *part) enumsOf(s map[string]any, path string, enums map[string][]string, root bool, seen map[string]bool) {
	if s["$ref"] == toneRef {
		return
	}
	if values, ok := s["enum"].([]any); ok {
		for _, v := range values {
			if value, ok := v.(string); ok {
				enums[path] = append(enums[path], value)
			}
		}
	}
	hs, other := p.sub(s, seen)
	if other && !root {
		return
	}
	for _, h := range hs {
		at := path
		if h.key != "" {
			at += "." + h.key
		}
		p.enumsOf(h.schema, at, enums, false, seen)
	}
}

// drawnTones are the tones a part's boxes are drawn in: the classes of tones, and the kinds of element
// (a tag, a part and the other classes) drawn in a tone and in none.
type drawnTones struct {
	found, toned, plain map[string]bool
}

// read reads the box n and the elements it holds but the parts it holds, whose tones are theirs.
func (d *drawnTones) read(n *html.Node) {
	elements := []*html.Node{n}
	var hold func(*html.Node)
	hold = func(p *html.Node) {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			if nodeAttr(c, "data-floe-part") == "" {
				elements = append(elements, c)
				hold(c)
			}
		}
	}
	hold(n)
	for _, e := range elements {
		if e.Type != html.ElementNode {
			continue
		}
		kind := []string{e.Data, nodeAttr(e, "data-floe-part")}
		tone := ""
		for _, c := range strings.Fields(nodeAttr(e, "class")) {
			if slices.Contains(tones, c) {
				tone = c
			} else {
				kind = append(kind, c)
			}
		}
		k := strings.Join(kind, " ")
		if tone != "" {
			d.found[tone], d.toned[k] = true, true
		} else {
			d.plain[k] = true
		}
	}
}

// has reports whether the part is drawn in tone, "" for none: an element of a kind drawn in a tone
// without one.
func (d *drawnTones) has(tone string) bool {
	if tone != "" {
		return d.found[tone]
	}
	for k := range d.toned {
		if d.plain[k] {
			return true
		}
	}
	return false
}

// readOccurrences adds to written each value written in the data of each part in v, a data file read, as
// valuesIn tells: a part is an object of one key, its name, in a list of parts.
func readOccurrences(v any, parts map[string]*part, written map[string]map[string]bool) {
	switch v := v.(type) {
	case map[string]any:
		for k, w := range v {
			if list, ok := w.([]any); ok && k == "parts" {
				for _, item := range list {
					if one, ok := item.(map[string]any); ok && len(one) == 1 {
						for name, data := range one {
							if parts[name] != nil {
								if written[name] == nil {
									written[name] = map[string]bool{}
								}
								maps.Copy(written[name], valuesIn(data, ""))
							}
						}
					}
				}
			}
			readOccurrences(w, parts, written)
		}
	case []any:
		for _, w := range v {
			readOccurrences(w, parts, written)
		}
	}
}

// valuesIn returns each string written in v, the data of a part at path, as path=value, path the keys it
// is written at from the part's (rows.cells.mark), but in the lists of the parts it holds.
func valuesIn(v any, path string) map[string]bool {
	found := map[string]bool{}
	switch v := v.(type) {
	case string:
		found[path+"="+v] = true
	case map[string]any:
		for k, w := range v {
			if k != "parts" {
				maps.Copy(found, valuesIn(w, path+"."+k))
			}
		}
	case []any:
		for _, w := range v {
			maps.Copy(found, valuesIn(w, path))
		}
	}
	return found
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// A schema whose $defs refer to themselves (a tree of nodes) is walked to its end, its enums and tones found.
func TestPartWalkEndsOnSelfReference(t *testing.T) {
	t.Parallel()

	var schema map[string]any
	if err := json.Unmarshal([]byte(`{"$ref": "#/$defs/node", "$defs": {"node": {"type": "object", "properties": {
		"kind": {"enum": ["leaf", "branch"]}, "tone": {"$ref": "floe:///parts/tone.json"},
		"kids": {"type": "array", "items": {"$ref": "#/$defs/node"}}}}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	p := &part{schema: schema, css: ""}
	enums := map[string][]string{}
	p.enumsOf(schema, "", enums, true, map[string]bool{})
	if !p.takesTone(schema, true, map[string]bool{}) || !slices.Equal(enums[".kind"], []string{"leaf", "branch"}) {
		t.Errorf("enums %v", enums)
	}
}
