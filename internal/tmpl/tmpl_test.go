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
	if n := bytes.Count(page, []byte("<b>0</b> ")); n != 4 {
		t.Errorf("%d counts of 0 apart from their words, want 4", n)
	}
}

// handoff's arrow stands between the one who hands over and the one who takes, and only when both are written.
func TestDrawHandoffHands(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ hands, want string }{
		{"from: Mina\nto: Kenji\n", `<span class="hands">Mina → Kenji</span>`},
		{"from: Mina\n", `<span class="hands">Mina</span>`},
		{"to: Kenji\n", `<span class="hands">Kenji</span>`},
	} {
		src := []byte("template: handoff\nlang: en\ntitle: x\ndate: 2030-05-14\n" + tt.hands)
		page, err := tmpl.Draw(floeKit(t), "handoff.yaml", src, tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
		if err != nil {
			t.Fatal(told(err))
		}
		if !bytes.Contains(page, []byte(tt.want)) {
			t.Errorf("%q draws no %s", tt.hands, tt.want)
		}
	}
}

// adr stamps each state in its own class and word, and shows the record that superseded it only when it is one.
func TestDrawAdrStates(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ state, word string }{
		{"proposed", "Proposed"}, {"accepted", "Accepted"}, {"rejected", "Rejected"},
		{"deprecated", "Deprecated"}, {"superseded", "Superseded"},
	} {
		src := "template: adr\nlang: en\ntitle: x\nstatus: " + tt.state + "\ncontext: c\ndecision: d\n"
		if tt.state == "superseded" {
			src += "superseded_by: ADR 0012\n"
		}
		page, err := tmpl.Draw(floeKit(t), "adr.yaml", []byte(src), tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
		if err != nil {
			t.Fatal(told(err))
		}
		if want := `<p class="status ` + tt.state + `">` + tt.word + `</p>`; !bytes.Contains(page, []byte(want)) {
			t.Errorf("%s draws no %s", tt.state, want)
		}
		if got := bytes.Contains(page, []byte(`class="superseded"`)); got != (tt.state == "superseded") {
			t.Errorf("%s: the record superseding it drawn = %v", tt.state, got)
		}
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
