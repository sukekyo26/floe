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
		{"source: true\nexample: nope\n", nil, "s.yaml:2:10: no example nope; use architecture, aws, class, er, flow, git, sequence, state, timeline, venn"},
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

// source: true shows the spec as written under the diagram, escaped; an example's, its file with its comments.
func TestSource(t *testing.T) {
	t.Parallel()

	spec := "source: true\nwidth: 200\nheight: 80\ntitle: </code></pre><script>x</script>\nparts: []\n"
	got, mistakes := draw(t, spec, drawWith("ja", nil, "the text\n<b>"))
	if mistakes != "" {
		t.Fatal(mistakes)
	}
	want := `</div><details class="dg-source"><summary>この図の spec</summary><pre><code>the text
&lt;b&gt;</code></pre></details>`
	if !strings.HasSuffix(got, want) {
		t.Errorf("drawn\n%s\nwant it to end\n%s", got, want)
	}
	if strings.Contains(got, "<script>") {
		t.Error("the title is not escaped")
	}
	plain, _ := draw(t, "width: 200\nheight: 80\ntitle: t\nparts: []\n", drawWith("en", nil, "the text"))
	if strings.Contains(plain, "dg-source") {
		t.Error("a spec without source: true shows its source")
	}
	got, _ = draw(t, "example: flow\nsource: true\n", drawWith("en", nil, "example: flow"))
	if !strings.Contains(got, "<summary>The spec of this diagram</summary><pre><code># Decision flow:") ||
		!strings.Contains(got, "title: { en: What floe publish checks") {
		t.Error("an example's source is not its file")
	}
}
