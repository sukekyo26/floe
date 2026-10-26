package tmpl_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// someIcons are the icons of a set kept: aws/res/aws-lambda alone, or, with failed, a set that cannot be read.
type someIcons struct{ failed error }

func (s someIcons) Lookup(name string) ([]byte, error) {
	switch {
	case s.failed != nil:
		return nil, s.failed
	case name == "aws/res/aws-lambda":
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), nil
	case strings.HasPrefix(name, "aws/"):
		return nil, icons.ErrNoIcon
	}
	return nil, icons.ErrNoSet
}

// diagramPage is a report of the diagrams given, each a spec in flow style, under lang.
func diagramPage(lang string, specs ...string) string {
	page := "lang: " + lang + "\ntemplate: report\ntitle: t\nsections:\n  - heading: h\n    parts:\n"
	for _, s := range specs {
		page += "      - diagram: " + s + "\n"
	}
	return page
}

const (
	boxSpec  = `{width: 300, height: 100, title: {en: Flow, ja: 流れ}, parts: [{box: {x: 20, y: 20, w: 80, h: 40, rows: [A]}}, {edge: {from: [100, 40], to: [200, 40]}}]}`
	iconSpec = `{width: 300, height: 100, title: i, parts: [{icon: {icon: aws/res/aws-lambda, x: 10, y: 10}}]}`
)

func drawPage(t *testing.T, k *kit.Kit, src string, o tmpl.Options) string {
	t.Helper()
	page, err := tmpl.Draw(k, "d.yaml", []byte(src), o)
	if err != nil {
		t.Fatalf("Draw: %s", told(err))
	}
	return string(page)
}

// Each diagram drawn is in the box of the part diagram after the script that zooms it, with ids of its own: the
// same spec twice too. A page of none has no script.
func TestDiagramsInPage(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	page := drawPage(t, k, diagramPage("en", boxSpec, boxSpec), none)
	if n := strings.Count(page, `<div data-floe-part="diagram"><script>`); n != 2 {
		t.Errorf("%d diagrams in their box after the script, want 2", n)
	}
	ids := regexp.MustCompile(`id="(dg\d+-[0-9a-f]+)_`).FindAllStringSubmatch(page, -1)
	if len(ids) != 2 || ids[0][1] == ids[1][1] {
		t.Errorf("ids %v, want two that differ", ids)
	}
	if !strings.Contains(page, `@scope ([data-floe-part="diagram"])`) {
		t.Error("the page holds no CSS of the part diagram")
	}
	plain := drawPage(t, k, "lang: en\ntemplate: report\ntitle: t\nsections:\n  - heading: h\n    parts:\n      - text: x\n", none)
	if strings.Contains(plain, "zoomable") {
		t.Error("a page of no diagram holds the zoom script")
	}
}

// A diagram's text is in the page's language, whatever the case of its tag or _ for -.
func TestDiagramLanguage(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"ja", "JA", "ja_JP"} {
		page := drawPage(t, floeKit(t), diagramPage(lang, boxSpec), none)
		if !strings.Contains(page, `aria-label="流れ"`) {
			t.Errorf("lang %s: the diagram is not drawn in Japanese", lang)
		}
	}
}

// A diagram's mistakes are at their place in the data file: those of its layout under the diagram's value, icons
// not kept at the diagram that has one, each such diagram once.
func TestDiagramMistakes(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	overlap := `{width: 300, height: 100, title: o, parts: [{box: {x: 20, y: 20, w: 80, h: 40, rows: [A]}}, ` +
		`{box: {x: 40, y: 30, w: 80, h: 40, rows: [B]}}]}`
	src := diagramPage("en", overlap, boxSpec, iconSpec, iconSpec)
	// At the box key of the item /parts/0, and at the first key of each diagram with an icon.
	want := lines(
		`d.yaml:7:63: box "A" overlaps box "B"`,
		`d.yaml:9:19: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path`,
		`d.yaml:10:19: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path`,
	)
	if got := drawn(t, k, src, none); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	o := none
	o.Icons = someIcons{failed: nil}
	if got := drawn(t, k, diagramPage("en", iconSpec, boxSpec), o); got != "" {
		t.Errorf("icons kept: told %s", got)
	}
}

// What fails in drawing a diagram that is no mistake is floe's failure, not one told at the diagram.
func TestDiagramFailure(t *testing.T) {
	t.Parallel()

	o := none
	o.Icons = someIcons{failed: errors.New("boom")} //nolint:err113 // a failure of the test's
	_, err := tmpl.Draw(floeKit(t), "d.yaml", []byte(diagramPage("en", iconSpec)), o)
	var found usererr.List
	if !errors.Is(err, kit.ErrFunc) || found.Add(err) == nil {
		t.Errorf("Draw = %v, want floe's failure", err)
	}
}

// A part of one's own holds diagrams by floe's schema of them, told at their place; its part.html may draw its
// own value as one with the function diagram, in the part diagram's box and CSS, or be told the value's mistakes.
func TestOwnPartDiagrams(t *testing.T) {
	t.Parallel()

	k := ownKit(t, map[string]string{
		"two/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"a": {"$ref": "floe:///parts/diagram.json"}, "b": {"$ref": "floe:///parts/diagram.json"}}}`,
		"two/part.html":   `<div data-floe-part="two">{{.a}}{{.b}}</div>`,
		"two/part.css":    "",
		"pic/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"pic/part.html":   `<div data-floe-part="pic">{{diagram}}</div>`,
		"pic/part.css":    "",
	}, mine(
		"<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>\n"+
			"{{.Data.two}}{{.Data.pic}}\n</body></html>\n",
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"two": {"$ref": "floe:///parts/two.json"}, "pic": {"$ref": "floe:///parts/pic.json"}}}`, nil))

	src := "lang: en\ntemplate: mine\ntwo:\n  a: " + boxSpec + "\n  b: " + boxSpec + "\npic: " + boxSpec + "\n"
	page := drawPage(t, k, src, none)
	if n := strings.Count(page, `<div data-floe-part="diagram"><script>`); n != 3 {
		t.Errorf("%d diagrams in their box, want 3", n)
	}
	if ids := regexp.MustCompile(`id="dg(\d+)-`).FindAllStringSubmatch(page, -1); len(ids) != 3 ||
		ids[0][1] == ids[1][1] || ids[1][1] == ids[2][1] {
		t.Errorf("ids %v, want three that differ", ids)
	}
	if !strings.Contains(page, `@scope ([data-floe-part="diagram"])`) {
		t.Error("the page holds no CSS of the part diagram")
	}

	// The layout of a diagram the part holds, under its key; the value the part draws as one, under its own.
	overlap := `{width: 300, height: 100, title: o, parts: [{box: {x: 20, y: 20, w: 80, h: 40, rows: [A]}}, ` +
		`{box: {x: 40, y: 30, w: 80, h: 40, rows: [B]}}]}`
	for _, tt := range []struct{ src, want string }{
		{"lang: en\ntemplate: mine\ntwo:\n  a: " + boxSpec + "\n  b: " + overlap + "\n", `d.yaml:5:51: box "A" overlaps box "B"`},
		{"lang: en\ntemplate: mine\npic: {x: 1}\n", lines(
			`d.yaml:3:7: additional properties 'x' not allowed`,
			`d.yaml:3:7: missing properties 'width', 'height', 'title', 'parts'`,
		)},
	} {
		if got := drawn(t, k, tt.src, none); got != tt.want {
			t.Errorf("told\n%s\nwant\n%s", got, tt.want)
		}
	}
}
