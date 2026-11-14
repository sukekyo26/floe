package cli_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"floe/internal/servetest"
)

// The output floe task guide shows in its text blocks is what floe task prints, run in order on a store as the guide's
// examples take it: tasks #130 and #131 open in project floe, the page design-notes on #130 and old-notes on none.
func TestTaskGuideExamples(t *testing.T) {
	t.Parallel()

	page := "lang: en\ntemplate: pic\n"
	s := servetest.Start(t, map[string]string{
		"templates/pic/page.html":     "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>T</title></head><body></body></html>\n",
		"templates/pic/schema.json":   `{"type": "object"}`,
		"templates/pic/example.yaml":  page,
		"home/floe/design-notes.yaml": page,
		"home/floe/old-notes.yaml":    page,
		"home/floe/api-notes.yaml":    page,
	})
	dir := filepath.Join(s.Dir, "home", "floe")
	run := func(args ...string) string {
		t.Helper()
		_, out, errs := runIn(dir, s.Env, args...)
		return out + errs
	}
	ready := func(args ...string) {
		t.Helper()
		if code, out, errs := runIn(dir, s.Env, args...); code != 0 {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
	}
	for _, slug := range []string{"design-notes", "old-notes", "api-notes"} {
		ready("publish", slug+".yaml")
	}
	ready("task", "--task", "#131", "--title", "Price table")
	ready("task", "--slug", "design-notes", "--task", "#130", "--title", "Fix login", "--state", "doing",
		"--url", "https://example.com/issues/482", "--category", "designs")

	doc, err := os.ReadFile("../../docs/agents/tasks.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?ms)^```text\n(.*?)^```").FindAllStringSubmatch(string(doc), -1)
	if len(blocks) < 2 {
		t.Fatalf("tasks.md: %d text blocks", len(blocks))
	}
	command := regexp.MustCompile(`"[^"]*"|\S+`)
	for _, b := range blocks {
		var args []string
		var want strings.Builder
		check := func() {
			if args == nil {
				return
			}
			if got := run(args...); got != want.String() {
				t.Errorf("floe %s = %q; tasks.md shows %q", strings.Join(args, " "), got, want.String())
			}
		}
		for line := range strings.Lines(b[1]) {
			cmd, ok := strings.CutPrefix(line, "$ floe ")
			if !ok {
				want.WriteString(line)
				continue
			}
			check()
			args = command.FindAllString(strings.TrimSpace(cmd), -1)
			for i, a := range args {
				args[i] = strings.Trim(a, `"`)
			}
			want.Reset()
		}
		check()
	}
}
