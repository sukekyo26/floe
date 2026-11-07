package server_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// The tasks are asked by the query, whose name and "no project" are the "/" a path could not hold; what cannot be a row
// is refused as the user's, what is not from the page itself or not of the form as a protocol's; a task with a page on
// it is not forgotten, and none is not found.
func TestWorkAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	task := func(project, name string) string {
		return "/api/tasks?" + url.Values{"project": {project}, "task": {name}}.Encode()
	}
	patch := func(path, body string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodPatch, path, body, "Origin", origin, "Content-Type", jsonType)
		return w.Code, w.Body.String()
	}
	tasks := func() store.Tasks {
		t.Helper()
		w := e.do(t, http.MethodGet, "/api/tasks", "")
		var all store.Tasks
		if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil || w.Code != http.StatusOK {
			t.Fatalf("GET /api/tasks = %d, %s, %v", w.Code, w.Body, err)
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
	if got := tasks()["shop"]["#1"]; got.State != "todo" {
		t.Errorf("trimmed, made todo: %+v", tasks())
	}
	if code, body := patch(task("shop", "#c"), `{"parent":"#1"}`); code != http.StatusNoContent {
		t.Errorf("a parent = %d, %s", code, body)
	}
	if got := tasks()["shop"]["#c"].Parent; got != "#1" {
		t.Errorf("the parent told: %q", got)
	}
	if code, body := patch(task("shop", "#1"), `{"parent":"#1"}`); code != http.StatusBadRequest || !strings.Contains(body, "its own parent") {
		t.Errorf("its own parent = %d, %s", code, body)
	}
	if code, body := patch(task("shop", "#1"), `{"new":true}`); code != http.StatusConflict || body != "task #1 is there already" {
		t.Errorf("new, of one there = %d, %q", code, body)
	}
	if code, body := patch(task("shop", "#fresh"), `{"new":false}`); code != http.StatusConflict || body != "task #fresh is no longer there" {
		t.Errorf("there, of none = %d, %q", code, body)
	}
	if code, _ := patch(task("shop", "#fresh"), `{"new":true,"title":"t"}`); code != http.StatusNoContent {
		t.Errorf("new = %d", code)
	}
	// What the screen saw of the parent: a parent made with the child, one moved meanwhile, and the fields alone.
	if code, body := patch(task("shop", "#fresh"), `{"new":false,"parent":"#made","parentWas":"","parentNew":true,"parentTitle":"m"}`); code != http.StatusNoContent || tasks()["shop"]["#made"].Title != "m" || tasks()["shop"]["#fresh"].Parent != "#made" {
		t.Errorf("a parent made = %d, %s", code, body)
	}
	if code, body := patch(task("shop", "#fresh"), `{"parent":"","parentWas":"#1"}`); code != http.StatusConflict || body != "the parent of task #fresh has changed since it was read" {
		t.Errorf("a parent moved meanwhile = %d, %q", code, body)
	}
	if code, body := patch(task("shop", "#fresh"), `{"parentTitle":"m"}`); code != http.StatusBadRequest || !strings.Contains(body, "go with a parent") {
		t.Errorf("a title alone = %d, %q", code, body)
	}
	if code, _ := patch(task("shop", "#fresh"), `{"parent":"","parentWas":"#made"}`); code != http.StatusNoContent {
		t.Errorf("taken off = %d", code)
	}
	if code, _ := patch(task("shop", "#none"), `{"parent":""}`); code != http.StatusNotFound {
		t.Errorf("a parent off no row = %d", code)
	}
	if code, body := patch(task("shop", "#c"), `{"parent":""}`); code != http.StatusNoContent || tasks()["shop"]["#c"].Parent != "" {
		t.Errorf("off = %d, %s, %v", code, body, tasks()["shop"]["#c"])
	}
	for _, c := range []struct {
		path, body, want string
	}{
		{"/api/tasks?task=x", `{}`, `a task needs its project: "/" for none`},
		{task("  ", "x"), `{}`, `a task needs its project: "/" for none`},
		{"/api/tasks?project=p", `{}`, "a task needs an ID"},
		{task("p", "x"), `{"state":"nope"}`, `unknown state "nope": use todo, doing, waiting, done, archived`},
		{task("p", "x"), `{"url":"javascript:alert(1)"}`, `"javascript:alert(1)" is not an http or https URL`},
		{"/api/tasks?project=p&task=%FF", `{}`, "a task's project and ID must be UTF-8"},
		{"/api/tasks?project=%FF&task=x", `{}`, "a task's project and ID must be UTF-8"},
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
	edit(`{"project":"shop","work":{"task":"#1","step":"s"}}`)
	forget := func(project, name string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodDelete, task(project, name), "", "Origin", origin)
		return w.Code, w.Body.String()
	}
	w := e.do(t, http.MethodPatch, "/api/artifacts/d", `{"work":{"task":"   "}}`, "Origin", origin, "Content-Type", jsonType)
	if w.Code != http.StatusBadRequest || w.Body.String() != "a task needs an ID" {
		t.Errorf("a name of spaces = %d, %s", w.Code, w.Body)
	}
	w = e.do(t, http.MethodDelete, "/api/tasks?project=shop&task=%FF", "", "Origin", origin)
	if w.Code != http.StatusBadRequest || w.Body.String() != "a task's project and ID must be UTF-8" {
		t.Errorf("forget of no UTF-8 = %d, %s", w.Code, w.Body)
	}
	if code, body := forget("shop", "#1"); code != http.StatusBadRequest || body != "task #1 of project shop still has 1 page(s), such as d: take them off it or remove them first" {
		t.Errorf("a task with a page = %d, %q", code, body)
	}
	if code, _ := forget("shop", "#2"); code != http.StatusNotFound {
		t.Errorf("no task = %d", code)
	}
	edit(`{"work":{"task":""}}`)
	if code, _ := forget("shop", "#1"); code != http.StatusNoContent {
		t.Errorf("a task of no page = %d", code)
	}

	// What it cannot read it says, and writes nothing over: a link of another scheme goes no further than here.
	file := filepath.Join(e.root, "tasks.json")
	for _, text := range []string{"null", `{"p":{"x":{"state":"doing","at":"2026-01-02T00:00:00Z","url":"javascript:alert(1)"}}}`} {
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if w := e.do(t, http.MethodGet, "/api/tasks", ""); w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), `"url"`) {
			t.Errorf("GET of %s = %d, %s", text, w.Code, w.Body)
		}
	}
	if err := os.WriteFile(file, []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := e.do(t, http.MethodGet, "/api/tasks", ""); w.Code != http.StatusBadRequest {
		t.Errorf("GET of a broken file = %d, %s", w.Code, w.Body)
	}
	if code, _ := patch(task("p", "x"), `{}`); code != http.StatusBadRequest {
		t.Errorf("PATCH over a broken file = %d", code)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "{ broken" {
		t.Errorf("a broken file came to be %s, %v", b, err)
	}
}
