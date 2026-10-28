package diagram_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// awsIcons are the icons the example aws names, as a set fetched holds them.
var awsIcons = iconsOf{
	"aws/amazon-bedrock": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "aws/group/aws-cloud-logo": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
	"aws/group/region": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "aws/res/client": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
}

func drawWith(lang string, icons diagram.Icons, source string) func(d *diagram.Drawer, v any) (string, error) {
	return func(d *diagram.Drawer, v any) (string, error) {
		svg, err := d.Draw(v, diagram.Options{Lang: i18n.Lang(lang), Icons: icons, Seq: 1, Source: source})
		return string(svg), err
	}
}

// Each built-in example is drawn by its name, in the language asked, as its spec is.
func TestExampleByName(t *testing.T) {
	t.Parallel()

	examples, err := diagram.Examples()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range examples {
		for _, lang := range []string{"en", "ja"} {
			byName, mistakes := draw(t, "example: "+e.Name+"\n", drawWith(lang, awsIcons, ""))
			if mistakes != "" {
				t.Fatalf("%s in %s: %s", e.Name, lang, mistakes)
			}
			bySpec, _ := draw(t, string(e.Spec), drawWith(lang, awsIcons, ""))
			if byName != bySpec {
				t.Errorf("%s in %s: drawn by name differs from its spec", e.Name, lang)
			}
		}
	}
}

// A name there is none of is told at the example's value; the mistakes of icons, which the icons kept decide, as the
// user's: an icon's name at the diagram, one of the icons' files at it, one of the whole output at no place.
func TestExampleMistakes(t *testing.T) {
	t.Parallel()

	broken := failing{usererr.NewAt("/i/aws/index.json", 1, 1, "icons_index_no_version", "aws")}
	for _, tt := range []struct {
		src   string
		icons diagram.Icons
		want  string
	}{
		{"# c\nexample: nope\n", nil, "s.yaml:2:10: no example nope; use architecture, aws, class, er, flow, git, sequence, state, timeline, venn"},
		// A diagram shows no spec.
		{"example: flow\nsource: true\n", nil, "additional properties 'source' not allowed"},
		{"source: true\nwidth: 200\nheight: 80\ntitle: t\nparts: []\n", nil, "additional properties 'source' not allowed"},
		{"example: aws\n", nil, "floe: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path"},
		{"example: aws\n", broken, `/i/aws/index.json:1:1: holds no version of the icons; run "floe icons fetch aws --force" to fetch them again`},
		{"example: aws\n", lambda, `s.yaml:1:1: icon "aws/amazon-bedrock": no icon "aws/amazon-bedrock" (search with: floe icons search <word>)`},
	} {
		_, got := draw(t, tt.src, drawWith("en", tt.icons, ""))
		if !strings.Contains(got, tt.want) {
			t.Errorf("%q: told\n%s\nwant it to hold\n%s", tt.src, got, tt.want)
		}
	}
	// What fails in reading the icons is floe's failure, not a mistake.
	_, err := drawer(t).Draw(map[string]any{"example": "aws"}, diagram.Options{
		Lang: "en", Icons: failing{err: errors.New("boom")}, Seq: 1, Source: "",
	}) //nolint:err113 // the test's
	var found usererr.List
	if err == nil || found.Add(err) == nil {
		t.Errorf("Draw = %v, want floe's failure", err)
	}
}
