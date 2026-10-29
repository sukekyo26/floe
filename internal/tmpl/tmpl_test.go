package tmpl_test

import (
	"bytes"
	"regexp"
	"strings"
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

// postmortem draws each severity and each state of an action in its own class and word.
func TestDrawPostmortem(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ lang, level, levelWord, state, stateWord string }{
		{"en", "high", "High", "todo", "To do"},
		{"en", "medium", "Medium", "doing", "In progress"},
		{"en", "low", "Low", "done", "Done"},
		{"ja", "high", "重大", "todo", "これから"},
		{"ja", "medium", "中程度", "doing", "作業中"},
		{"ja", "low", "軽微", "done", "済み"},
	} {
		page := drawPostmortem(t, "lang: "+tt.lang+"\nseverity: "+tt.level+"\nactions: [{what: w, state: "+tt.state+"}]\n")
		for _, want := range []string{`<span class="severity ` + tt.level + `">` + tt.levelWord + `</span>`, `<span class="state ` + tt.state + `">` + tt.stateWord + `</span>`} {
			if !bytes.Contains(page, []byte(want)) {
				t.Errorf("%s %s %s draws no %s", tt.lang, tt.level, tt.state, want)
			}
		}
	}
}

// postmortem draws no badge for a severity left out, and the owner and state columns of its actions only when an
// action has one; an action without a state has an empty cell.
func TestDrawPostmortemLeftOut(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, data string
		want, not  []string
	}{
		{
			"no severity", "lang: en\nactions: [{what: w, state: done}]\n",
			[]string{`<th>State</th>`, `<span class="state done">`},
			[]string{`class="severity`, `<th>Owner</th>`},
		},
		{
			"no state", "lang: en\nseverity: low\nactions: [{what: w, owner: o}]\n",
			[]string{`<span class="severity low">`, `<th>Owner</th>`, `<td>o</td>`},
			[]string{`class="state`, `<th>State</th>`},
		},
		{
			"some states", "lang: ja\nactions: [{what: a, state: done}, {what: b}]\n",
			[]string{`<th>状態</th>`, `<td><span class="state done">済み</span></td>`, `<td>b</td><td></td>`},
			[]string{`<th>担当</th>`},
		},
	} {
		page := drawPostmortem(t, tt.data)
		for _, want := range tt.want {
			if !bytes.Contains(page, []byte(want)) {
				t.Errorf("%s draws no %s", tt.name, want)
			}
		}
		for _, not := range tt.not {
			if bytes.Contains(page, []byte(not)) {
				t.Errorf("%s draws %s", tt.name, not)
			}
		}
	}
}

// drawPostmortem draws a postmortem of the required keys and data.
func drawPostmortem(t *testing.T, data string) []byte {
	t.Helper()

	src := "template: postmortem\ntitle: x\nsummary: s\ntimeline: [{date: '10:42', title: t}]\ncause: c\n" + data
	page, _, err := tmpl.Draw(floeKit(t), "pm.yaml", []byte(src), tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
	if err != nil {
		t.Fatal(told(err))
	}
	return page
}

// runbook draws a step of its rollback as one of its steps, its owner badge beside the heading, which takes its id
// from the title alone; what is left out has no section.
func TestDrawRunbook(t *testing.T) {
	t.Parallel()

	step := "{title: Restart, who: Ops, text: t, run: {steps: [{run: ls}]}, if_fails: f}"
	ids := regexp.MustCompile(` id="[^"]*"`)
	for _, tt := range []struct{ lang, fails string }{{"en", "If it fails"}, {"ja", "失敗したら"}} {
		page := drawRunbook(t, "lang: "+tt.lang+"\nneeds: [n]\ncaution: c\nsteps: ["+step+"]\nrollback: ["+step+"]\n")
		lists := strings.Split(string(page), `<ol class="steps">`)
		if len(lists) != 3 {
			t.Fatalf("%s: %d lists of steps, want 2", tt.lang, len(lists)-1)
		}
		steps, back := lists[1][:strings.Index(lists[1], "</ol>")], lists[2][:strings.Index(lists[2], "</ol>")]
		if ids.ReplaceAllString(steps, "") != ids.ReplaceAllString(back, "") {
			t.Errorf("%s: a step of the rollback\n%s\nis not drawn as one of the steps\n%s", tt.lang, back, steps)
		}
		for _, want := range []string{`<h3 id="restart">Restart</h3><span class="who">Ops</span>`, `<b>` + tt.fails + `</b>`, `class="needs"`, `class="caution"`} {
			if !strings.Contains(steps+string(page), want) {
				t.Errorf("%s draws no %s", tt.lang, want)
			}
		}
	}
	page := drawRunbook(t, "lang: en\nsteps: [{title: t}]\n")
	for _, not := range []string{`class="rollback"`, `class="needs"`, `class="caution"`, `class="who"`, `class="fails"`} {
		if bytes.Contains(page, []byte(not)) {
			t.Errorf("left out, %s is drawn", not)
		}
	}
}

// drawRunbook draws a runbook of data.
func drawRunbook(t *testing.T, data string) []byte {
	t.Helper()

	page, _, err := tmpl.Draw(floeKit(t), "rb.yaml", []byte("template: runbook\ntitle: x\n"+data), tmpl.Options{Theme: "", Untitled: "", Images: nil, ImagesDir: ""})
	if err != nil {
		t.Fatal(told(err))
	}
	return page
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
