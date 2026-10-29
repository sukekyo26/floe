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
	page, _, err := tmpl.Draw(floeKit(t), "handoff.yaml", src, tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
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
		page, _, err := tmpl.Draw(floeKit(t), "handoff.yaml", src, tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
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

	for _, tt := range []struct{ lang, state, word, by string }{
		{"en", "proposed", "Proposed", ""},
		{"en", "accepted", "Accepted", ""},
		{"en", "rejected", "Rejected", ""},
		{"en", "deprecated", "Deprecated", ""},
		{"en", "superseded", "Superseded", "Superseded by"},
		{"ja", "proposed", "提案", ""},
		{"ja", "accepted", "採用", ""},
		{"ja", "rejected", "却下", ""},
		{"ja", "deprecated", "廃止", ""},
		{"ja", "superseded", "置き換え済み", "置き換えた記録"},
	} {
		src := "template: adr\nlang: " + tt.lang + "\ntitle: x\nstatus: " + tt.state + "\ncontext: c\ndecision: d\n"
		if tt.by != "" {
			src += "superseded_by: ADR 0012\n"
		}
		page, _, err := tmpl.Draw(floeKit(t), "adr.yaml", []byte(src), tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
		if err != nil {
			t.Fatal(told(err))
		}
		if want := `<p class="status ` + tt.state + `">` + tt.word + `</p>`; !bytes.Contains(page, []byte(want)) {
			t.Errorf("%s %s draws no %s", tt.lang, tt.state, want)
		}
		by := `<p class="superseded-by">` + tt.by + `: `
		if got := bytes.Contains(page, []byte(`class="superseded-by"`)); got != (tt.by != "") || got && !bytes.Contains(page, []byte(by)) {
			t.Errorf("%s %s: the record superseding it drawn = %v, want %v as %s", tt.lang, tt.state, got, tt.by != "", by)
		}
	}
}

// postmortem draws each severity and each state of an action in its own class and word; one left out has no badge.
func TestDrawPostmortem(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ lang, level, levelWord, state, stateWord string }{
		{"en", "high", "High", "todo", "To do"},
		{"en", "medium", "Medium", "doing", "In progress"},
		{"en", "low", "Low", "done", "Done"},
		{"ja", "high", "重大", "todo", "これから"},
		{"ja", "medium", "中程度", "doing", "作業中"},
		{"ja", "low", "軽微", "done", "済み"},
		{"en", "", "", "", ""},
	} {
		src := "template: postmortem\nlang: " + tt.lang + "\ntitle: x\nsummary: s\ntimeline: [{date: '10:42', title: t}]\ncause: c\n"
		if tt.level != "" {
			src += "severity: " + tt.level + "\nactions: [{what: w, state: " + tt.state + "}]\n"
		} else {
			src += "actions: [{what: w}]\n"
		}
		page, _, err := tmpl.Draw(floeKit(t), "pm.yaml", []byte(src), tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
		if err != nil {
			t.Fatal(told(err))
		}
		if tt.level == "" {
			if bytes.Contains(page, []byte(`class="severity`)) || bytes.Contains(page, []byte(`class="state`)) {
				t.Error("a severity or a state left out is drawn")
			}
			continue
		}
		for _, want := range []string{`<span class="severity ` + tt.level + `">` + tt.levelWord + `</span>`, `<span class="state ` + tt.state + `">` + tt.stateWord + `</span>`} {
			if !bytes.Contains(page, []byte(want)) {
				t.Errorf("%s %s %s draws no %s", tt.lang, tt.level, tt.state, want)
			}
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
			if _, _, err := tmpl.Draw(k, name+".yaml", tpl.Example(), tmpl.Options{Theme: theme, Untitled: "", ImagesDir: ""}); err != nil {
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
	first, _, err := tmpl.Draw(k, "report.yaml", tpl.Example(), o)
	if err != nil {
		t.Fatal(told(err))
	}
	second, _, err := tmpl.Draw(k, "report.yaml", tpl.Example(), o)
	if err != nil {
		t.Fatal(told(err))
	}
	if !bytes.Equal(first, second) {
		t.Error("two drawings of report's example differ")
	}
}
