package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"

	"floe/internal/cli"
	"floe/internal/guard"
)

// testLimit is the memory a guarded floe of these tests may hold: above what drawing a right page takes with the
// race detector, far below what the pages that repeat a long text take.
const testLimit = 128 << 20

// The pages these tests draw that repeat a text of 100,000 bytes 5,000 times: about 500 MB, which ends them even
// with no watch, so that a watch that fails fails the test and not the machine.
const (
	repeatTemplate = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": {"n": {"type": "integer"}, "intro": {"type": "string"}, "rows": {"type": "array"}}}`
	repeatPage = "<!doctype html><html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>" +
		"<body>{{range .Data.rows}}<p>{{$.Data.intro}}</p>{{end}}</body></html>"
	repeatComputed = `copies: 'let s = repeat("x", 100000); len(map(1..(n ?? 0), s + "x"))'` + "\n"
)

// guardedFloe runs this test binary as floe in a process of its own from dir, its memory watched with testLimit,
// with the settings vars, and returns its exit code and stderr.
func guardedFloe(t *testing.T, dir string, vars map[string]string, args ...string) (int, string) {
	t.Helper()

	return guardedFloeWith(t, testLimit, dir, vars, args...)
}

// guardedFloeWith is guardedFloe, its memory watched with limit.
func guardedFloeWith(t *testing.T, limit int, dir string, vars map[string]string, args ...string) (int, string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, selfBin, args...)
	cmd.Dir = dir
	cmd.Env = []string{asCLI + "=1", guarded + "=" + strconv.Itoa(limit), "GOMEMLIMIT=2GiB", "PATH=" + os.Getenv("PATH")}
	for k, v := range vars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode(), stderr.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, stderr.String()
}

// A command that draws a page holding more memory than floe draws with stops, telling it at the file it drew (exit
// code 1), whether an expression or page.html makes it; a right page is drawn, and floe run in a test's process
// watches nothing.
func TestDrawMemory(t *testing.T) {
	t.Parallel()

	intro := strings.Repeat("x", 100000)
	rows := strings.Repeat("  - 1\n", 5000)
	dir := write(t, map[string]string{
		"templates/many/schema.json":   repeatTemplate,
		"templates/many/page.html":     repeatPage,
		"templates/many/computed.yaml": repeatComputed,
		"templates/many/example.yaml":  "template: many\nlang: en\nn: 5000\n",
		"computed.yaml":                "template: many\nlang: en\ntitle: T\nn: 5000\n",
		"page.yaml":                    "template: many\nlang: en\ntitle: T\nintro: " + intro + "\nrows:\n" + rows,
		"right.yaml":                   good,
	})
	vars := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{
		"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates"), "FLOE_PARTS_DIR": filepath.Join(dir, "parts"),
		"FLOE_THEMES_DIR": filepath.Join(dir, "themes"),
	})
	for _, d := range []string{"parts", "themes", "home"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const stopped = ": drawing it needed more than 128 MiB of memory"
	for _, c := range []struct {
		args []string
		file string
	}{
		{[]string{"check", "right.yaml", "computed.yaml"}, "computed.yaml"},
		{[]string{"check", "page.yaml"}, "page.yaml"},
		{[]string{"render", "--out-dir", "out", "page.yaml"}, "page.yaml"},
		{[]string{"template", "check"}, "example.yaml"},
	} {
		code, stderr := guardedFloe(t, dir, vars, c.args...)
		if !strings.Contains(stderr, c.file+stopped) || code != 1 {
			t.Errorf("floe %v = %d, %q; want %q at %s and 1", c.args, code, stderr, stopped, c.file)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("floe render that stopped wrote %v", err)
	}
	// The right page of every part, with its images, is drawn within the limit. Its stderr is not held to be empty:
	// under coverage, the process says GOCOVERDIR is not set.
	every := filepath.Join("..", "corpus", "testdata", "every-part")
	if code, stderr := guardedFloe(t, every, vars, "check", "every-part.yaml"); code != 0 || strings.Contains(stderr, stopped) {
		t.Errorf("floe check every-part.yaml = %d, %q", code, stderr)
	}

	limit := debug.SetMemoryLimit(-1)
	if code, _, _ := run(dir, "check", "right.yaml"); code != 0 {
		t.Errorf("floe check right.yaml in process = %d", code)
	}
	if now := debug.SetMemoryLimit(-1); now != limit {
		t.Errorf("floe in a test's process set the memory limit to %d, from %d", now, limit)
	}
}

// A page drawn just under tmpl.MaxPage is drawn, and one just over it refused at its data file, alike by floe check
// and by the server's floe __draw, both within the memory floe draws with: 780 and 880 rows of a text of 100,000
// bytes, 78 and 88 MB of HTML.
func TestPageSize(t *testing.T) {
	t.Parallel()

	if underRace {
		// A page of 78 MB takes the race detector past the server's 30 seconds, and a hundred seconds for the test.
		t.Skip("pages near the size floe draws are too slow to draw under the race detector")
	}

	e := newDenv(t)
	page := func(rows int) string {
		return "template: many\nlang: en\ntitle: T\nintro: " + strings.Repeat("x", 100000) + "\nrows:\n" +
			strings.Repeat("  - 1\n", rows)
	}
	for name, text := range map[string]string{
		"templates/many/schema.json":  repeatTemplate,
		"templates/many/page.html":    repeatPage,
		"templates/many/example.yaml": "template: many\nlang: en\n",
		"under.yaml":                  page(780),
		"over.yaml":                   page(880),
		"parts/.keep":                 "",
		"themes/.keep":                "",
	} {
		path := filepath.Join(e.tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	e.vars["FLOE_THEMES_DIR"] = filepath.Join(e.tmp, "themes")
	e.vars[guarded] = strconv.Itoa(guard.Limit)
	const told = "over.yaml: the page drawn is 84 MiB, over the 80 MiB a page may be"
	for _, command := range []string{"check", "publish"} {
		if r := e.exe(t, selfBin, command, "under.yaml"); r.code != 0 || strings.Contains(r.stderr, "drawing it") {
			t.Errorf("floe %s under.yaml = %d, %q (%s)", command, r.code, r.stderr, r.why())
		}
		if r := e.exe(t, selfBin, command, "over.yaml"); r.code != 1 || !strings.Contains(r.stderr, told) {
			t.Errorf("floe %s over.yaml = %d, %q; want 1 and %q (%s)", command, r.code, r.stderr, told, r.why())
		}
	}
}
