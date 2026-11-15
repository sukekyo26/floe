package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// floe render draws one's own part in a section of report, and in one's own part holding any, which is compiled
// with any naming it.
func TestRenderOwnPartInReport(t *testing.T) {
	t.Parallel()

	const schema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", `
	dir := write(t, map[string]string{
		"parts/quote/schema.json": schema + `"required": ["text"], "properties": {"text": {"type": "string"}}}`,
		"parts/quote/part.html":   `<q data-floe-part="quote">{{.text}}</q>`,
		"parts/quote/part.css":    "",
		"parts/box/schema.json":   schema + `"properties": {"items": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}}`,
		"parts/box/part.html":     `<div data-floe-part="box">{{range .items}}{{range .}}{{.}}{{end}}{{end}}</div>`,
		"parts/box/part.css":      "",
		"r.yaml": "template: report\nlang: en\ntitle: R\nsections:\n  - heading: a\n    parts:\n" +
			"      - quote: { text: alone }\n      - box: { items: [ { quote: { text: held } } ] }\n",
	})
	env := map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "parts")}
	if code, stdout, stderr := runEnv(dir, env, "render", "r.yaml"); code != 0 || stdout != "r.html\n" || stderr != "" {
		t.Fatalf("floe render = %d, %q, %q", code, stdout, stderr)
	}
	page, err := os.ReadFile(filepath.Join(dir, "r.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<q data-floe-part="quote">alone</q>`, `<div data-floe-part="box"><q data-floe-part="quote">held</q></div>`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("r.html holds no %s", want)
		}
	}
}
