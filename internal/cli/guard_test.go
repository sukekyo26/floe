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

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, selfBin, args...)
	cmd.Dir = dir
	cmd.Env = []string{asCLI + "=1", guarded + "=" + strconv.Itoa(testLimit), "GOMEMLIMIT=2GiB", "PATH=" + os.Getenv("PATH")}
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
	// The right page of every part, with its images, is drawn within the limit.
	every := filepath.Join("..", "corpus", "testdata", "every-part")
	if code, stderr := guardedFloe(t, every, vars, "check", "every-part.yaml"); code != 0 || stderr != "" {
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
