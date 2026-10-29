package tmpl_test

import (
	"bytes"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// floeKit returns floe's own parts, themes and templates.
func floeKit(t *testing.T) *kit.Kit {
	t.Helper()
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// told words err as the command line does.
func told(err error) string {
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

// handoff draws from its date alone: each count of a list left out is 0, and the sections left out are not drawn.
func TestDrawHandoffDateAlone(t *testing.T) {
	t.Parallel()

	src := []byte("template: handoff\nlang: en\ntitle: Quiet day\ndate: 2030-05-14\n")
	page, err := tmpl.Draw(floeKit(t), "handoff.yaml", src, tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
	if err != nil {
		t.Fatal(told(err))
	}
	for _, id := range []string{"asks", "done", "doing", "next", "notes"} {
		if bytes.Contains(page, []byte(`id="`+id+`"`)) {
			t.Errorf("the section %s, left out, is drawn", id)
		}
	}
	if n := bytes.Count(page, []byte("<b>0</b>")); n != 4 {
		t.Errorf("%d counts of 0, want 4", n)
	}
}

// The example.yaml of each of floe's templates is drawn with every theme of floe. Pages drawn as develop drew
// them are internal/corpus's.
func TestDrawExamples(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	_, _, themes, templates := k.Names()
	for _, name := range templates {
		tpl, _ := k.Template(name)
		for _, theme := range themes {
			if _, err := tmpl.Draw(k, name+".yaml", tpl.Example(), tmpl.Options{Theme: theme, Untitled: "", ImagesDir: ""}); err != nil {
				t.Errorf("%s in %s: %s", name, theme, told(err))
			}
		}
	}
}

// A page drawn twice from one input is the same bytes, its items of format html, diagrams and scripts too: no
// mark of a drawing is left in it, so that drawing a stored page again keeps it when nothing changed.
func TestDrawTwiceSame(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	tpl, _ := k.Template("report")
	o := tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""}
	first, err := tmpl.Draw(k, "report.yaml", tpl.Example(), o)
	if err != nil {
		t.Fatal(told(err))
	}
	second, err := tmpl.Draw(k, "report.yaml", tpl.Example(), o)
	if err != nil {
		t.Fatal(told(err))
	}
	if !bytes.Equal(first, second) {
		t.Error("two drawings of report's example differ")
	}
}
