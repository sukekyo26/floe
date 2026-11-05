package server_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// The tasks are asked by the query, whose name and "no project" are the "/" a path could not hold; what cannot be a row
// is refused as the user's, what is not from the page itself or not of the form as a protocol's; a task with a page on
// it is not forgotten, and none is not found.
func TestWorkAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	task := func(project, item string) string {
		return "/api/work?" + url.Values{"project": {project}, "item": {item}}.Encode()
	}
	patch := func(path, body string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodPatch, path, body, "Origin", origin, "Content-Type", jsonType)
		return w.Code, w.Body.String()
	}
	tasks := func() store.Tasks {
		t.Helper()
		w := e.do(t, http.MethodGet, "/api/work", "")
		var all store.Tasks
		if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil || w.Code != http.StatusOK {
			t.Fatalf("GET /api/work = %d, %s, %v", w.Code, w.Body, err)
		}
		return all
	}
	if got := tasks(); len(got) != 0 {
		t.Errorf("no tasks = %v", got)
	}
	if code, body := patch(task("/", "#a/b"), `{"state":"waiting","step":"s"}`); code != http.StatusNoContent || body != "" {
		t.Errorf("a name of / = %d, %s", code, body)
	}
	if got := tasks()["/"]["#a/b"]; got.State != "waiting" || got.Step != "s" {
		t.Errorf("no project's task: %+v", got)
	}
	if code, _ := patch(task(" shop ", " #1 "), `{}`); code != http.StatusNoContent {
		t.Errorf("an empty write = %d", code)
	}
	if got := tasks()["shop"]["#1"]; got.State != "doing" {
		t.Errorf("trimmed, made doing: %+v", tasks())
	}
	for _, c := range []struct {
		path, body, want string
	}{
		{"/api/work?item=x", `{}`, `a task needs its project: "/" for none`},
		{task("  ", "x"), `{}`, `a task needs its project: "/" for none`},
		{"/api/work?project=p", `{}`, "a task needs a name"},
		{task("p", "x"), `{"state":"nope"}`, `unknown state "nope": use waiting, doing, done`},
		{task("p", "x"), `{"url":"javascript:alert(1)"}`, `"javascript:alert(1)" is not an http or https URL`},
	} {
		if code, body := patch(c.path, c.body); code != http.StatusBadRequest || body != c.want {
			t.Errorf("PATCH %s %s = %d, %q, want %q", c.path, c.body, code, body, c.want)
		}
	}
	if _, ok := tasks()["p"]; ok {
		t.Errorf("a refused write wrote: %v", tasks())
	}
	if code, _ := patch(task("p", "x"), `{"bad":1}`); code != http.StatusBadRequest {
		t.Errorf("a key of none = %d", code)
	}
	want(t, e.do(t, http.MethodPatch, task("p", "x"), `{}`, "Content-Type", jsonType), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodPatch, task("p", "x"), `{}`, "Origin", "null", "Content-Type", jsonType), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodDelete, task("shop", "#1"), "", "Origin", "null"), http.StatusForbidden, "forbidden origin")

	e.publish(t, "d", noted("one"), nil)
	edit := func(body string) {
		t.Helper()
		if w := e.do(t, http.MethodPatch, "/api/artifacts/d", body, "Origin", origin, "Content-Type", jsonType); w.Code != http.StatusOK {
			t.Fatalf("PATCH d %s = %d, %s", body, w.Code, w.Body)
		}
	}
	edit(`{"project":"shop","work":{"item":"#1","step":"s"}}`)
	forget := func(project, item string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodDelete, task(project, item), "", "Origin", origin)
		return w.Code, w.Body.String()
	}
	if code, body := forget("shop", "#1"); code != http.StatusBadRequest || body != "task #1 of project shop still has 1 page(s), such as d: take them off it or remove them first" {
		t.Errorf("a task with a page = %d, %q", code, body)
	}
	if code, _ := forget("shop", "#2"); code != http.StatusNotFound {
		t.Errorf("no task = %d", code)
	}
	edit(`{"work":{"item":""}}`)
	if code, _ := forget("shop", "#1"); code != http.StatusNoContent {
		t.Errorf("a task of no page = %d", code)
	}

	// What it cannot read it says, and writes nothing over.
	file := filepath.Join(e.root, "work.json")
	if err := os.WriteFile(file, []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := e.do(t, http.MethodGet, "/api/work", ""); w.Code != http.StatusBadRequest {
		t.Errorf("GET of a broken file = %d, %s", w.Code, w.Body)
	}
	if code, _ := patch(task("p", "x"), `{}`); code != http.StatusBadRequest {
		t.Errorf("PATCH over a broken file = %d", code)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "{ broken" {
		t.Errorf("a broken file came to be %s, %v", b, err)
	}
}
