package tmpl_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// A fence in a text part is warned of at the value, in a section and in the stack of details and tabs; code in a
// line and an indented block are not. The page is drawn.
func TestWarnFence(t *testing.T) {
	t.Parallel()

	data := "lang: en\ntemplate: report\nsections:\n  - parts:\n" +
		"      - text: \"```sh\\nls\\n```\"\n" +
		"      - text: \"```x``` in a line\"\n" +
		"      - text: \"Para.\\n\\n    indented\"\n" +
		"      - details: {title: d, parts: [{text: \"~~~\\nin details\\n~~~\"}]}\n" +
		"      - tabs: [{title: a, parts: [{text: \"```\\nin tabs\\n```\"}]}, {title: b, parts: [{text: plain}]}]\n"
	page, warnings, err := tmpl.Draw(floeKit(t), "d.yaml", []byte(data), none)
	if err != nil || page == nil {
		t.Fatalf("Draw = %d bytes, %s", len(page), told(err))
	}
	want := "d.yaml:5:15: warning: " + fence + "\nd.yaml:8:44: warning: " + fence + "\nd.yaml:9:42: warning: " + fence
	if got := laid(warnings); got != want {
		t.Errorf("warnings\n%s\nwant\n%s", got, want)
	}
}

// A fence in Markdown of a template's own, where no code part stands, is not warned of; a page with a mistake
// gives no warning.
func TestWarnNotHere(t *testing.T) {
	t.Parallel()

	schema := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"m": {"type": "string", "format": "markdown"}}}`
	k := ownKit(t, nil, mine(strings.Replace(minePage, "{{with .Data.note}}", "{{with .Data.m}}", 1), schema, nil))
	if _, warnings, err := tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\nm: \"```\\ncode\\n```\"\n"), none); err != nil || warnings != nil {
		t.Errorf("own Markdown: %v, %s", warnings, told(err))
	}
	data := "lang: en\ntemplate: report\nsections:\n  - parts:\n" +
		"      - text: \"```\\ncode\\n```\"\n" +
		"      - table: {columns: [{title: x}], rows: [[a, b]]}\n"
	if _, warnings, err := tmpl.Draw(floeKit(t), "d.yaml", []byte(data), none); err == nil || warnings != nil {
		t.Errorf("with a mistake: %v, %v", warnings, err)
	}
}

// floe's templates draw their examples without a warning.
func TestFloeExamplesWarnNot(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	_, _, _, templates := k.Names()
	for _, name := range templates {
		tp, _ := k.Template(name)
		if _, warnings, err := tmpl.Draw(k, name+"/example.yaml", tp.Example(), none); err != nil || warnings != nil {
			t.Errorf("%s: %s\n%s", name, laid(warnings), told(err))
		}
	}
}

const fence = "write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button"

// laid lays warnings out as the command line does.
func laid(warnings []*usererr.Error) string {
	return usererr.Report{Mistakes: nil, Failure: "", Warnings: usererr.Warn(i18n.EnglishCatalog(), warnings)}.Render("floe: ")
}
