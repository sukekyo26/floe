package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// floe part list tells of every part a line, a part that cannot be used after them; floe part show tells
// what a part takes and its items, the looks it takes, the parts it holds and its parts themes.
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
	if r := e.run(t, "", "part", "show", "table"); r.code != 0 || !strings.Contains(r.stdout, "\nholds: cell\nparts themes with a file of it: blueprint, brutal, ") ||
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
