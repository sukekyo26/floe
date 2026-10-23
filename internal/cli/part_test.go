package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// floe part list tells of every part a line, a part that cannot be used after them; floe part show tells
// what a part takes and its items, the looks it takes, the parts it holds and its themes.
func TestPartListAndShow(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	parts := filepath.Join(e.tmp, "parts")
	e.vars["FLOE_PARTS_DIR"] = parts
	if r := e.run(t, "", "part", "list"); r.code != 0 || !strings.HasPrefix(r.stdout, "bars\tbuilt-in\tValues side by side as bars") ||
		!strings.Contains(r.stdout, "\nrows\tbuilt-in\tRows in a panel") || r.stderr != "" {
		t.Errorf("part list = %+v", r)
	}
	if r := e.run(t, "", "part", "show", "risks"); r.code != 0 || r.stdout != `risks: Risks in a table, each how likely and how heavy (low, mid or high), coloured by both, and what is done about it
where: built-in
takes: a list of keys and values
looks: panel, table
items:
  fix: text — What is done about it; a line of Markdown
  impact: one of low, mid, high, required
  likely: one of low, mid, high, required
  what: text, required — A line of Markdown
` {
		t.Errorf("part show risks = %+v", r)
	}
	if r := e.run(t, "", "part", "show", "table"); r.code != 0 || !strings.Contains(r.stdout, "\nholds: cell\nthemes with a file of it: blueprint, brutal, ") ||
		!strings.Contains(r.stdout, "\n  columns: a list of keys and values, required\n    num: ") {
		t.Errorf("part show table = %+v", r)
	}
	if r := e.run(t, "", "part", "show", "rows"); r.code != 0 || strings.Contains(r.stdout, "takes:") || strings.Contains(r.stdout, "items:") ||
		!strings.Contains(r.stdout, "\nlooks: panel\n") {
		t.Errorf("part show of a look = %+v", r)
	}

	e.write(t, "parts/note/schema.json", `{"type": "object", "description": "My note", "properties": {"text": {"type": "string"}}}`)
	e.write(t, "parts/note/part.html", `<p data-floe-part="note">{{.text}}</p>`)
	e.write(t, "parts/note/part.css", "")
	e.write(t, "parts/broken/part.html", "")
	note := filepath.Join(parts, "note")
	if r := e.run(t, "", "part", "show", "note"); r.code != 0 || !strings.HasPrefix(r.stdout, "note: My note\nwhere: "+note+", in place of floe's note\ntakes: keys and values\nitems:\n  text: text\n") {
		t.Errorf("part show of one's own = %+v", r)
	}
	r := e.run(t, "", "part", "list")
	if r.code != 1 || !strings.Contains(r.stdout, "\nnote\t"+note+"\tMy note\n") || strings.Contains(r.stdout, "broken") ||
		!strings.Contains(r.stderr, "the part broken cannot be used") {
		t.Errorf("part list with one that cannot be used = %+v", r)
	}
	for name, want := range map[string]string{"broken": "the part broken cannot be used", "nope": `unknown part "nope"`} {
		if r := e.run(t, "", "part", "show", name); r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, want) {
			t.Errorf("part show %s = %+v", name, r)
		}
	}
}

// floe part new tells where the copy is, what of floe's it takes the place of, and the files of the themes
// it copied, into the directory of themes of one's own, made with those above it.
func TestPartNew(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	parts := filepath.Join(e.tmp, "parts")
	e.vars["FLOE_PARTS_DIR"] = parts
	if r := e.run(t, "", "part", "new", "note", "--from", "note"); r.code != 0 || r.stdout != "made the part note in "+filepath.Join(parts, "note")+", a copy of floe's note\n"+
		"it takes the place of floe's note\n"+
		"copied the files of its themes too: blueprint, brutal, default, editorial, ink, notebook, swiss, washi ("+filepath.Join(e.tmp, "home", ".config", "floe", "themes")+"/<theme>/note.css)\n" {
		t.Errorf("part new in place of floe's = %+v", r)
	}
	if r := e.run(t, "", "part", "new", "aside", "--from", "note"); r.code != 0 || !strings.HasPrefix(r.stdout, "made the part aside in "+filepath.Join(parts, "aside")+", a copy of note\ncopied the files of its themes too: ") {
		t.Errorf("part new of one's own = %+v", r)
	}
	if r := e.run(t, "", "part", "new", "aside"); r.code != 2 || !strings.Contains(r.stderr, "name the part to copy with --from") {
		t.Errorf("part new without --from = %+v", r)
	}
	if r := e.run(t, "", "part", "new", "aside", "--from", "note"); r.code != 1 || !strings.Contains(r.stderr, "aside is there already") {
		t.Errorf("part new of a name there already = %+v", r)
	}
}

// floe part check prints every mistake of the parts of one's own and exits 1, warns on stderr of what goes
// into no page, and prints nothing when they are right.
func TestPartCheck(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	e.vars["FLOE_THEMES_DIR"] = filepath.Join(e.tmp, "themes")
	if r := e.run(t, "", "part", "check"); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("part check without parts = %+v", r)
	}
	if r := e.run(t, "", "part", "new", "grid", "--from", "table"); r.code != 0 {
		t.Fatalf("part new = %+v", r)
	}
	if r := e.run(t, "", "part", "check"); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("part check of a copy = %+v", r)
	}
	e.write(t, "themes/default/nope.css", "")
	if r := e.run(t, "", "part", "check"); r.code != 0 || !strings.Contains(r.stderr, "nope.css: this file goes into no page") {
		t.Errorf("part check with a stray file = %+v", r)
	}
	e.write(t, "parts/grid/part.html", "<table></table>")
	e.write(t, "themes/mine/grid.css", "}\n")
	r := e.run(t, "", "part", "check")
	if r.code != 1 || !strings.Contains(r.stderr, "the part grid cannot be used") || !strings.Contains(r.stderr, "your own theme mine cannot be used") {
		t.Errorf("part check with mistakes = %+v", r)
	}
	if r := e.run(t, "", "part", "check", "parts"); r.code != 2 {
		t.Errorf("part check with an argument = %+v", r)
	}
}

// floe part check with FLOE_PARTS_DIR under a file says so, rather than check no part and pass.
func TestPartCheckUnderAFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "afile", "")
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(file, "parts")
	want(t, "part check with its directory under a file", e.run(t, "", "part", "check"), 1, "",
		"floe: FLOE_PARTS_DIR, "+file+"/parts, is under "+file+", which is not a directory; set FLOE_PARTS_DIR to a directory")
}
