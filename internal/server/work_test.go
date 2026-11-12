package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/store"
)

// A task is written by PATCH /api/work, of its project ("/" for none) and name; what cannot be a row is refused as the
// user's, what is not from the page itself or not of the form as a protocol's; a task with a page on it is not
// forgotten (by the query of DELETE /api/tasks), and none is not found.
func TestWorkAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	task := func(project, name string) string {
		return "/api/tasks?" + url.Values{"taskProject": {project}, "task": {name}}.Encode()
	}
	work := func(body string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodPatch, "/api/work", body, "Origin", origin, "Content-Type", jsonType)
		return w.Code, w.Body.String()
	}
	// A write of the task (project, name) alone, of the fields of body
	patch := func(project, name, body string) (int, string) {
		t.Helper()
		fields := map[string]any{}
		if err := json.Unmarshal([]byte(body), &fields); err != nil {
			t.Fatal(err)
		}
		fields["taskProject"], fields["task"] = project, name
		b, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return work(string(b))
	}
	// The first task of the answer of a write
	first := func(body string) store.TaskWritten {
		t.Helper()
		var w store.WorkWritten
		if err := json.Unmarshal([]byte(body), &w); err != nil || len(w.Tasks) == 0 {
			t.Fatalf("the answer %q: %v", body, err)
		}
		return w.Tasks[0]
	}
	// The answer of a write: the keys the row is under, and its rev
	answer := func(body string) string {
		t.Helper()
		w := first(body)
		return fmt.Sprintf("%s %s %d", w.TaskProject, w.Task, w.Row.Rev)
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
	// The answer is the row as written, under its keys
	if code, body := patch("/", "#a/b", `{"state":"done"}`); code != http.StatusOK || answer(body) != "/ #a/b 1" {
		t.Errorf("a name of / = %d, %s", code, body)
	}
	if got := tasks()["/"]["#a/b"]; got.State != "done" {
		t.Errorf("no project's task: %+v", got)
	}
	// A category is a page's alone: a task's body has none.
	if code, body := patch("shop", "#c", `{"category":"x"}`); code != http.StatusBadRequest {
		t.Errorf("a task's category = %d, %s", code, body)
	}
	// A write of nothing makes no row: not found.
	if code, body := patch(" shop ", " #1 ", `{}`); code != http.StatusNotFound {
		t.Errorf("an empty write of no row = %d, %s", code, body)
	}
	if code, body := patch(" shop ", " #1 ", `{"state":"todo"}`); code != http.StatusOK || answer(body) != "shop #1 1" {
		t.Errorf("a write = %d, %s", code, body)
	}
	if got := tasks()["shop"]["#1"]; got.State != "todo" {
		t.Errorf("trimmed, made todo: %+v", tasks())
	}
	// Whether the write made the row, and whether it wrote tasks.json: not for the row as it was
	madeWrote := func(body string) string {
		t.Helper()
		w := first(body)
		return fmt.Sprintf("%t %t", w.Made, w.Wrote)
	}
	for _, c := range []struct{ body, want string }{
		{`{"title":"t"}`, "true true"}, {`{"title":"t"}`, "false false"}, {`{"title":"u"}`, "false true"},
	} {
		if code, body := patch("shop", "#m", c.body); code != http.StatusOK || madeWrote(body) != c.want {
			t.Errorf("made and wrote of %s = %d, %s; want %s", c.body, code, body, c.want)
		}
	}
	if code, body := patch("shop", "#c", `{"parent":"#1"}`); code != http.StatusOK {
		t.Errorf("a parent = %d, %s", code, body)
	}
	if got := tasks()["shop"]["#c"].Parent; got != "#1" {
		t.Errorf("the parent told: %q", got)
	}
	if code, body := patch("shop", "#1", `{"parent":"#1"}`); code != http.StatusBadRequest || !strings.Contains(body, "its own parent") {
		t.Errorf("its own parent = %d, %s", code, body)
	}
	if code, body := patch("shop", "#1", `{"new":true}`); code != http.StatusConflict || body != "task #1 is there already" {
		t.Errorf("new, of one there = %d, %q", code, body)
	}
	if code, body := patch("shop", "#fresh", `{"new":false}`); code != http.StatusConflict || body != "task #fresh is no longer there" {
		t.Errorf("there, of none = %d, %q", code, body)
	}
	if code, _ := patch("shop", "#fresh", `{"new":true,"title":"t"}`); code != http.StatusOK {
		t.Errorf("new = %d", code)
	}
	// What the screen read (rev): a parent made with the child, a row written since, and the fields alone.
	if code, body := patch("shop", "#fresh", `{"new":false,"rev":1,"parent":"#made","parentNew":true,"parentTitle":"m"}`); code != http.StatusOK || answer(body) != "shop #fresh 2" || tasks()["shop"]["#made"].Title != "m" || tasks()["shop"]["#fresh"].Parent != "#made" {
		t.Errorf("a parent made = %d, %s", code, body)
	}
	if code, body := patch("shop", "#fresh", `{"parent":"","rev":1}`); code != http.StatusConflict || body != "task #fresh has changed since it was read" {
		t.Errorf("written since = %d, %q", code, body)
	}
	if code, body := patch("shop", "#fresh", `{"parent":"","parentWas":"#made"}`); code != http.StatusBadRequest || !strings.Contains(body, `unknown object member name "parentWas"`) {
		t.Errorf("parentWas = %d, %q", code, body)
	}
	if code, body := patch("shop", "#fresh", `{"parentTitle":"m"}`); code != http.StatusBadRequest || !strings.Contains(body, "go with a parent") {
		t.Errorf("a title alone = %d, %q", code, body)
	}
	if code, body := patch("shop", "#fresh", `{"parent":"","rev":2}`); code != http.StatusOK || answer(body) != "shop #fresh 3" {
		t.Errorf("taken off = %d, %s", code, body)
	}
	if code, _ := patch("shop", "#none", `{"parent":""}`); code != http.StatusNotFound {
		t.Errorf("a parent off no row = %d", code)
	}
	if code, body := patch("shop", "#c", `{"parent":""}`); code != http.StatusOK || tasks()["shop"]["#c"].Parent != "" {
		t.Errorf("off = %d, %s, %v", code, body, tasks()["shop"]["#c"])
	}
	for _, c := range []struct {
		body, want string
	}{
		{`{"task":"x","state":"done"}`, `a task needs its project: "/" for none`},
		{`{"taskProject":"  ","task":"x","state":"done"}`, `a task needs its project: "/" for none`},
		{`{"taskProject":"p","state":"done"}`, "a task needs an ID"},
		{`{"taskProject":"p","task":"x","state":"nope"}`, `unknown state "nope": use todo, doing, done, archived`},
		{`{"taskProject":"p","task":"x","url":"javascript:alert(1)"}`, `"javascript:alert(1)" is not an http or https URL`},
	} {
		if code, body := work(c.body); code != http.StatusBadRequest || body != c.want {
			t.Errorf("PATCH %s = %d, %q, want %q", c.body, code, body, c.want)
		}
	}
	// A project or ID that is no UTF-8, a lone surrogate in the body, is refused as decodeStrict reads it, before the store
	for _, body := range []string{`{"taskProject":"p","task":"\ud800","state":"done"}`, `{"taskProject":"\ud800","task":"x","state":"done"}`, `{"slugs":["d"],"task":"\ud800"}`} {
		if code, answer := work(body); code != http.StatusBadRequest || !strings.Contains(answer, "surrogate") {
			t.Errorf("PATCH %s = %d, %q", body, code, answer)
		}
	}
	if _, ok := tasks()["p"]; ok {
		t.Errorf("a refused write wrote: %v", tasks())
	}
	if code, _ := patch("p", "x", `{"bad":1}`); code != http.StatusBadRequest {
		t.Errorf("a key of none = %d", code)
	}
	want(t, e.do(t, http.MethodPatch, "/api/work", `{}`, "Content-Type", jsonType), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodPatch, "/api/work", `{}`, "Origin", "null", "Content-Type", jsonType), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodDelete, task("shop", "#1"), "", "Origin", "null"), http.StatusForbidden, "forbidden origin")

	e.publish(t, "d", noted("one"), nil)
	edit := func(body string) string {
		t.Helper()
		w := e.do(t, http.MethodPatch, "/api/pages/d", body, "Origin", origin, "Content-Type", jsonType)
		if w.Code != http.StatusOK {
			t.Fatalf("PATCH d %s = %d, %s", body, w.Code, w.Body)
		}
		return strings.TrimSpace(w.Body.String())
	}
	edit(`{"project":"shop"}`)
	// The answer of a page's work is its slug, project and work as written
	page := func(body string) string {
		t.Helper()
		code, answer := work(body)
		var w store.WorkWritten
		if err := json.Unmarshal([]byte(answer), &w); err != nil || code != http.StatusOK || len(w.Pages) != 1 {
			t.Fatalf("PATCH %s = %d, %s", body, code, answer)
		}
		b, err := json.Marshal(w.Pages[0])
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := page(`{"slugs":["d"],"task":"#1","category":" s "}`); got != `{"slug":"d","taskProject":"shop","work":{"task":"#1","category":"s"}}` {
		t.Errorf("the answer of a page's work: %s", got)
	}
	forget := func(project, name string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodDelete, task(project, name), "", "Origin", origin)
		return w.Code, w.Body.String()
	}
	if code, body := work(`{"slugs":["d"],"task":"   "}`); code != http.StatusBadRequest || body != "a task needs an ID" {
		t.Errorf("a name of spaces = %d, %s", code, body)
	}
	// A page's work is written by PATCH /api/work alone: its edit has no such key
	if w := e.do(t, http.MethodPatch, "/api/pages/d", `{"work":{"task":"#1"}}`, "Origin", origin, "Content-Type", jsonType); w.Code != http.StatusBadRequest {
		t.Errorf("a page's work by its edit = %d, %s", w.Code, w.Body)
	}
	w := e.do(t, http.MethodDelete, "/api/tasks?taskProject=shop&task=%FF", "", "Origin", origin)
	if w.Code != http.StatusBadRequest || w.Body.String() != "a task's project and ID must be UTF-8" {
		t.Errorf("forget of no UTF-8 = %d, %s", w.Code, w.Body)
	}
	if code, body := forget("shop", "#1"); code != http.StatusBadRequest || body != "task #1 of project shop still has 1 page(s), such as d: take them off it or delete them first" {
		t.Errorf("a task with a page = %d, %q", code, body)
	}
	if code, _ := forget("shop", "#2"); code != http.StatusNotFound {
		t.Errorf("no task = %d", code)
	}
	if got := page(`{"slugs":["d"],"task":""}`); got != `{"slug":"d","taskProject":"shop"}` {
		t.Errorf("the answer of an edit taking the page off its task: %s", got)
	}
	// The query's old key reads as no project, and removes nothing: the task is still there for the next to remove
	if w := e.do(t, http.MethodDelete, "/api/tasks?project=shop&task=%231", "", "Origin", origin); w.Code != http.StatusBadRequest {
		t.Errorf("forget by the old key = %d, %s", w.Code, w.Body)
	}
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
	if code, _ := patch("p", "x", `{}`); code != http.StatusBadRequest {
		t.Errorf("PATCH over a broken file = %d", code)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "{ broken" {
		t.Errorf("a broken file came to be %s, %v", b, err)
	}
}

// PATCH /api/work reads the keys the management page sends of a task, the embedded TaskEdit's as the write's own, and
// refuses as a protocol's what only a sender's mistake makes.
func TestWorkWriteAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	patch := func(body string) (int, string) {
		t.Helper()
		w := e.do(t, http.MethodPatch, "/api/work", body, "Origin", origin, "Content-Type", jsonType)
		return w.Code, w.Body.String()
	}
	code, body := patch(`{"taskProject":"p","task":"#2","new":true,"title":"C","parent":"#P","parentNew":true,"parentTitle":"Par"}`)
	var got store.WorkWritten
	if err := json.Unmarshal([]byte(body), &got); err != nil || code != http.StatusOK {
		t.Fatalf("a child and its parent made = %d, %s, %v", code, body, err)
	}
	// The answer names the task's project as the pages' are named (Edited): the Go type reads either, the text does not
	if !strings.Contains(body, `"taskProject":"p"`) || strings.Contains(body, `"project"`) {
		t.Errorf("the answer names the project by a key but taskProject: %s", body)
	}
	all, err := store.New(e.root).ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if c, p := all["p"]["#2"], all["p"]["#P"]; c.Title != "C" || c.Parent != "#P" || p.Title != "Par" || len(got.Tasks) != 1 || !got.Tasks[0].Made {
		t.Errorf("made: %+v, %+v, answered %+v", c, p, got)
	}
	if code, body = patch(`{"taskProject":"p","task":"#2","new":false,"rev":1,"state":"doing"}`); code != http.StatusOK {
		t.Errorf("a screen's write, as it saw the row = %d, %s", code, body)
	}
	for _, c := range []struct{ body, want string }{
		{`{"taskProject":"p","task":"#2","category":"x"}`, "category, pin and clear go with slugs"},
		{`{"slugs":["a"],"taskProject":"p"}`, "taskProject goes without slugs"},
		// The old key is no other name for taskProject: refused as a key unknown, whether alone or beside it
		{`{"project":"p","task":"#2","new":true,"title":"t"}`, `unknown object member name "project"`},
		{`{"project":"a","taskProject":"b","task":"#2","new":true,"title":"t"}`, `unknown object member name "project"`},
	} {
		if code, body := patch(c.body); code != http.StatusBadRequest || !strings.Contains(body, c.want) {
			t.Errorf("%s = %d, %s", c.body, code, body)
		}
	}
}
