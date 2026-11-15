package kit

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/i18n"
)

// The docs for agents quote what floe's parts, templates and functions make as they make it: each text below, made from
// floe, stands in its doc as many times as said, the doc read alone.
func TestAgentDocQuotes(t *testing.T) {
	t.Parallel()

	bytes, err := formatBytes(1536)
	if err != nil {
		t.Fatal(err)
	}
	marks := "`" + strings.Join(markKinds[:len(markKinds)-1], "`, `") + "` or `" + markKinds[len(markKinds)-1] + "`"
	unknown, _, _ := strings.Cut(i18n.EnglishCatalog().Msg("kit_asset_unknown", ".x/a.png"), ": ")
	var code struct {
		Properties struct {
			Start struct{ Minimum, Maximum float64 } `json:"start"`
		} `json:"properties"`
	}
	schemaOf(t, "floe/parts/code/schema.json", &code)
	var diagram struct {
		Defs struct {
			Graph struct {
				Properties struct {
					Edges struct {
						MaxItems int `json:"maxItems"`
					} `json:"edges"`
				} `json:"properties"`
			} `json:"graph"`
		} `json:"$defs"`
	}
	schemaOf(t, "floe/parts/diagram/schema.json", &diagram)
	for _, q := range []struct {
		file, text string
		times      int
	}{
		{"templates.md", "`{{.Calc.total \\| num}}` is `" + numOf(t, 1234567) + "`", 1},
		{"templates.md", "`{{.Calc.rate \\| num 3}}` is `" + numOf(t, 3, 0.0064812) + "`", 1},
		{"templates.md", "`{{.Calc.size \\| bytes}}` is `" + bytes + "`", 1},
		{"parts.md", "1536 is `" + bytes + "`", 1},
		{"templates.md", "at most " + strconv.Itoa(maxImage>>20) + " MiB", 1},
		{"templates.md", unknown, 1},
		{"parts.md", marks, 2},
		{"parts.md", "from " + numOf(t, code.Properties.Start.Minimum) + " to " + numOf(t, code.Properties.Start.Maximum), 1},
		{"diagrams.md", "more than " + strconv.Itoa(diagram.Defs.Graph.Properties.Edges.MaxItems) + " edges", 1},
	} {
		src, err := os.ReadFile("../../docs/agents/" + q.file)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(src), q.text); got != q.times {
			t.Errorf("docs/agents/%s holds %q %d times, floe makes it %d: write it as floe makes it", q.file, q.text, got, q.times)
		}
	}
}

// schemaOf reads the schema file into v, for the values a doc copies from it.
func schemaOf(t *testing.T, file string, v any) {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

// The blocks of code the docs copy from floe's files are copies: each line of templates.md's css block is a line of
// report's page.css, and parts.md's json block is the items of bars' schema without their descriptions.
func TestAgentDocCopies(t *testing.T) {
	t.Parallel()

	page, err := os.ReadFile("floe/templates/report/page.css")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.Lines(string(page)) {
		lines = append(lines, strings.TrimSpace(line))
	}
	css := docBlocks(t, "templates.md", "css")
	if len(css) != 1 {
		t.Fatalf("docs/agents/templates.md has %d css blocks, want 1", len(css))
	}
	for line := range strings.Lines(css[0]) {
		if line = strings.TrimSpace(line); line != "" && !slices.Contains(lines, line) {
			t.Errorf("docs/agents/templates.md: the css block shows %q, which report's page.css does not hold", line)
		}
	}

	blocks := docBlocks(t, "parts.md", "json")
	if len(blocks) != 1 {
		t.Fatalf("docs/agents/parts.md has %d json blocks, want 1", len(blocks))
	}
	var shown struct {
		Items any `json:"items"`
	}
	if err := json.Unmarshal([]byte("{"+blocks[0]+"}"), &shown); err != nil {
		t.Fatalf("docs/agents/parts.md: the json block is not \"items\": {…}: %v", err)
	}
	var bars struct {
		Items any `json:"items"`
	}
	schemaOf(t, "floe/parts/bars/schema.json", &bars)
	if want := withoutDescriptions(bars.Items); !reflect.DeepEqual(shown.Items, want) {
		t.Errorf("docs/agents/parts.md: the json block is not bars' items without their descriptions:\n%v\nwant\n%v", shown.Items, want)
	}
}

// docBlocks returns the text of each block of code of lang in docs/agents/<file>.
func docBlocks(t *testing.T, file, lang string) []string {
	t.Helper()
	var out []string
	for n := range agentDoc(t, file).Descendants() {
		if n.DataAtom == atom.Code && slices.ContainsFunc(n.Attr, func(a html.Attribute) bool { return a.Key == "class" && a.Val == "language-"+lang }) {
			out = append(out, textOf(n))
		}
	}
	return out
}

// withoutDescriptions returns v, a JSON value, with the key description taken out of every object in it.
func withoutDescriptions(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			if k != "description" {
				out[k] = withoutDescriptions(item)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = withoutDescriptions(item)
		}
		return out
	}
	return v
}
