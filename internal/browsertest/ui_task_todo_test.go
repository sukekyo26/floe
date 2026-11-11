package browsertest_test

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// To do lists the latest to come to it first; narrow, the row of views folds so that every view and the way to the
// filters, of a long project's name, show on the screen without overlapping.
func TestUITaskTodo(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	const long = "社内ポータルの作り直しと権限の見直しと監査の記録"
	for _, name := range []string{"#a", "#b"} {
		s.floe(t, "work", "--project", long, "--task", name, "--state", "todo")
	}
	s.floe(t, "work", "--project", long, "--task", "#w", "--state", "done")
	// Times apart, which two writes in a row may not be: tied, the names would give the order looked for
	file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	text, err := os.ReadFile(file) //nolint:gosec // G304: a path the test names
	if err != nil {
		t.Fatal(err)
	}
	text = regexp.MustCompile(`("#a": \{\n\s*"state": "todo",\n\s*"at": )"[^"]+"`).ReplaceAll(text, []byte(`${1}"2026-01-01T00:00:00.000Z"`))
	text = regexp.MustCompile(`("#b": \{\n\s*"state": "todo",\n\s*"at": )"[^"]+"`).ReplaceAll(text, []byte(`${1}"2026-01-02T00:00:00.000Z"`))
	if err = os.WriteFile(file, text, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	if got := strings.Count(string(text), "2026-01-0"); got != 2 {
		t.Fatalf("times written: %d in %s", got, text)
	}
	list := "/tasks?" + url.Values{"project": {long}}.Encode()
	b.navigate(list + "&view=todo")
	b.until("the rows", "document.querySelectorAll('#list > li .tname').length === 2")
	if got := b.eval("[...document.querySelectorAll('#list > li .tname')].map((e) => e.textContent).join(' ')"); got != "#b #a" {
		t.Errorf("to do: %q", got)
	}

	defer b.unsize()
	for _, width := range []int64{360, 400, 672} {
		b.size(width, 800, width < 600)
		b.navigate(list)
		b.until("the views", "document.querySelector('.shelf .views button[data-view=todo]')")
		b.still()
		got := b.eval(`(() => {
			const all = [...document.querySelectorAll('.shelf .views button[data-view], #narrow-filter')].filter((e) => e.offsetParent);
			const r = all.map((e) => e.getBoundingClientRect());
			const out = r.filter((x) => x.left < 0 || x.right > innerWidth).length;
			let over = 0;
			for (let i = 0; i < r.length; i++) for (let j = i + 1; j < r.length; j++) {
				if (Math.min(r[i].right, r[j].right) - Math.max(r[i].left, r[j].left) > 1 && Math.min(r[i].bottom, r[j].bottom) - Math.max(r[i].top, r[j].top) > 1) over++;
			}
			return [all.length, out, over, document.documentElement.scrollWidth > innerWidth].join(' ');
		})()`)
		if want := fmt.Sprintf("%d 0 0 false", 5); got != want {
			t.Errorf("width %d: shown, off the screen, overlapping, wider = %s, want %s", width, got, want)
		}
	}
}
