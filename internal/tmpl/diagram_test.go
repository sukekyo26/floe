package tmpl_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/tmpl"
)

const twoDiagrams = `template: report
sections:
  - parts:
      - diagram:
          id: a
          width: 100
          height: 50
          title: The first
          parts:
            - dot: %s
      - diagram:
          id: %s
          width: 100
          height: 50
          title: The second
          parts: []
`

// The diagrams of a data file are checked as floe diagram render checks them, at the lines and columns
// of the data file, their ids across the page. A spec stops at its first mistake in shape.
func TestDiagramMistakes(t *testing.T) {
	t.Parallel()

	src := strings.Replace(strings.Replace(twoDiagrams, "%s", "{ cx: 0, cy: 0 }", 1), "%s", "a", 1) +
		"      - diagram:\n          id: b\n          width: 100\n          title: The third\n          parts: []\n"
	_, err := builtins.Read("data.yaml", []byte(src), "", "")
	want := "data.yaml:10:15: dot at 0,0 sticks out of the 100x50 canvas\n" +
		`data.yaml:12:11: diagram id "a" is already used in data.yaml:5; give each diagram its own id` + "\n" +
		"data.yaml:18:11: diagram: give height"
	if err == nil || err.Error() != want {
		t.Errorf("Read = %v\nwant %s", err, want)
	}
}

// Each diagram is drawn where page.html puts it, and the zoom goes once at the end of the page.
func TestDiagrams(t *testing.T) {
	t.Parallel()

	src := strings.Replace(strings.Replace(twoDiagrams, "%s", "{ cx: 50, cy: 25 }", 1), "%s", "b", 1)
	got := render(t, src)
	first, second := strings.Index(got, `aria-label="The first"`), strings.Index(got, `aria-label="The second"`)
	if first < 0 || second < first {
		t.Errorf("the diagrams are drawn at %d and %d:\n%s", first, second, got)
	}
	assets, err := diagram.Assets()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, assets); n != 1 || !strings.HasSuffix(got, assets+"\n") {
		t.Errorf("the page holds the zoom %d times, want once at its end", n)
	}
}

// A diagram shows the icons of the library's store.
func TestDiagramIcons(t *testing.T) {
	t.Parallel()

	src := strings.Replace(strings.Replace(twoDiagrams, "- dot: %s", "- icon: { x: 10, y: 5, name: aws/aws-lambda, size: 24 }", 1), "%s", "b", 1)
	d, err := tmpl.NewLibrary("", icons.New("../diagram/testdata/icons")).Read("data.yaml", []byte(src), "", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.Render("t", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `<use href="#a_icon-`) {
		t.Errorf("the icon is not drawn:\n%s", page)
	}
	// Without the icons fetched, the drawing stops and says how to fetch them.
	_, err = tmpl.NewLibrary("", icons.New(t.TempDir())).Read("data.yaml", []byte(src), "", "")
	if err == nil || !strings.Contains(err.Error(), "floe icons fetch aws") {
		t.Errorf("Read without the icons = %v", err)
	}
}
