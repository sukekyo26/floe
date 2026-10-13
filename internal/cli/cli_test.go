package cli_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/version"
)

// asCLI makes the test binary run as floe, so that the server a publish starts (os.Executable() serve)
// is this binary.
const asCLI = "FLOE_TEST_AS_CLI"

func TestMain(m *testing.M) {
	if os.Getenv(asCLI) == "1" {
		dir, _ := os.Getwd() //nolint:errcheck // as main does
		os.Exit(cli.Run(context.Background(), os.Args[1:], cli.Env{
			Getenv: os.Getenv, Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr, Environ: os.Environ(),
		}))
	}
	// The binary run as floe (another build, a started server) reads the real environment: English,
	// whatever the developer's locale.
	for k, v := range map[string]string{asCLI: "1", "FLOE_LANG": "en"} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	dir, err := os.MkdirTemp("", "floe-cli-test-")
	if err != nil {
		panic(err)
	}
	otherBuild, err = copySelf(dir)
	if err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir) //nolint:errcheck // a leftover temporary directory is harmless
	os.Exit(code)
}

// otherBuild is a copy of this test binary, as a reinstalled floe with another build.
var otherBuild string

// copySelf writes otherBuild. It runs before the parallel tests: a process they fork while the copy
// is open for writing would hold it, and running the copy would fail with ETXTBSY (golang/go#22315).
func copySelf(dir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err //nolint:wrapcheck // TestMain panics with it
	}
	b, err := os.ReadFile(self)
	if err != nil {
		return "", err //nolint:wrapcheck // TestMain panics with it
	}
	bin := filepath.Join(dir, "floe-copy")
	//nolint:gosec // G306: an executable
	return bin, os.WriteFile(bin, append(b, 0), 0o700) //nolint:wrapcheck // TestMain panics with it
}

// env is a store, a home and a free port in a temporary directory, with a browser that records what
// it is asked to open.
type env struct {
	tmp  string
	vars map[string]string
	port string // the port of its server, which a test may set FLOE_PORT away from
}

// handedOut are the ports given to the tests running now, so that two never share one: a port
// closed for a server to take later is free again, and the system may give it out once more.
var handedOut sync.Map

// freePort returns a port free now, for a server a test starts on it.
func freePort(t *testing.T) int {
	t.Helper()

	ln := listenFree(t)
	port := portOf(t, ln)
	if ln.Close() != nil {
		t.Fatal("no free port")
	}
	return port
}

// listenFree listens on a port given to no other test running now, and keeps it given until t ends.
func listenFree(t *testing.T) net.Listener {
	t.Helper()

	for range 100 {
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := portOf(t, ln)
		if _, taken := handedOut.LoadOrStore(port, true); !taken {
			t.Cleanup(func() { handedOut.Delete(port) })
			return ln
		}
		_ = ln.Close()
	}
	t.Fatal("no port free of the other tests")
	return nil
}

func portOf(t *testing.T, ln net.Listener) int {
	t.Helper()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("not a TCP listener")
	}
	return addr.Port
}

// hold listens on a port of its own for e and keeps it, so that nothing takes it between choosing it
// and listening on it.
func (e *env) hold(t *testing.T) net.Listener {
	t.Helper()

	ln := listenFree(t)
	e.port = strconv.Itoa(portOf(t, ln))
	e.vars["FLOE_PORT"] = e.port
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func newEnv(t *testing.T) *env {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(freePort(t))
	e := &env{tmp: tmp, port: port, vars: map[string]string{
		"HOME":      filepath.Join(tmp, "home"),
		"FLOE_DIR":  filepath.Join(tmp, "store"),
		"FLOE_PORT": port,
		"BROWSER":   filepath.Join(tmp, "browser"),
	}}
	e.write(t, "browser", "#!/bin/sh\nprintf '%s\\n' \"$1\" >>'"+filepath.Join(tmp, "opened")+"'\n")
	if err := os.Chmod(filepath.Join(tmp, "browser"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.vars["HOME"], 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.stop(t) })
	return e
}

func (e *env) base() string {
	return "http://127.0.0.1:" + e.vars["FLOE_PORT"]
}

// page writes a data file for the report template, titled title ("" for none).
func (e *env) page(t *testing.T, rel, title string) string {
	t.Helper()

	src := "template: report\n"
	if title != "" {
		src += "title: " + title + "\n"
	}
	return e.write(t, rel, src)
}

func (e *env) write(t *testing.T, rel, body string) string {
	t.Helper()

	path := filepath.Join(e.tmp, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type result struct {
	code           int
	stdout, stderr string
}

// run runs floe in process from dir (a directory under tmp, "" for tmp itself).
func (e *env) run(t *testing.T, dir string, args ...string) result {
	t.Helper()
	return e.runIn(t.Context(), t, dir, args...)
}

// stop stops the server a test left on its port, and fails the test when the server outlives it. It
// runs as a cleanup, after t.Context() is canceled, so it takes a context that is not.
func (e *env) stop(t *testing.T) {
	t.Helper()

	ctx := context.WithoutCancel(t.Context())
	e.vars["FLOE_PORT"] = e.port
	// A server in this process (floe serve, run by a test) stops with the test's context: a SIGTERM to
	// its pid would stop the test binary.
	if h, up := e.healthIn(ctx, t); up && h.PID != os.Getpid() {
		if r := e.runIn(ctx, t, "", "stop"); r.code != 0 {
			t.Errorf("stop = %+v", r)
		}
	}
	if !e.wait(ctx, t, false) {
		t.Error("the server outlived the test")
	}
}

// wait probes e's port until a server of floe is up there, or down when up is false, and reports
// whether it came to that. Starting and stopping take well under a second, but a loaded machine (the
// whole suite under -race) may take longer.
func (e *env) wait(ctx context.Context, t *testing.T, up bool) bool {
	t.Helper()

	for deadline := time.Now().Add(10 * time.Second); ; {
		if _, now := e.healthIn(ctx, t); now == up {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (e *env) runIn(ctx context.Context, t *testing.T, dir string, args ...string) result {
	t.Helper()

	cwd := filepath.Join(e.tmp, dir)
	if err := os.MkdirAll(cwd, 0o750); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(ctx, args, cli.Env{
		Getenv: func(k string) string { return e.vars[k] }, Dir: cwd, Stdout: &stdout, Stderr: &stderr,
		Environ: os.Environ(),
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// exe runs the binary bin as floe in a process of its own.
func (e *env) exe(t *testing.T, bin string, args ...string) result {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), bin, args...)
	cmd.Dir = e.tmp
	cmd.Env = os.Environ()
	for k, v := range e.vars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok { //nolint:errorlint // Run returns it unwrapped
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// opened returns what the browser stand-in recorded. The file exists a moment before the URL is in it,
// so it waits for a whole line.
func (e *env) opened(t *testing.T) string {
	t.Helper()

	for range 20 {
		if b, err := os.ReadFile(filepath.Join(e.tmp, "opened")); err == nil && bytes.HasSuffix(b, []byte("\n")) {
			return string(b)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}

func (e *env) health(t *testing.T) (server.Health, bool) {
	t.Helper()
	return e.healthIn(t.Context(), t)
}

func (e *env) healthIn(ctx context.Context, t *testing.T) (server.Health, bool) {
	t.Helper()

	p := daemon.Probe(ctx, e.base())
	return p.Health, p.State == daemon.Ours
}

func want(t *testing.T, what string, got result, code int, stdout, stderr string) {
	t.Helper()

	if got.code != code || got.stdout != stdout || !strings.Contains(got.stderr, stderr) || stderr == "" && got.stderr != "" {
		t.Errorf("%s = %d %q %q, want %d %q and stderr containing %q", what, got.code, got.stdout, got.stderr, code, stdout, stderr)
	}
}

func TestRun(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	v := "floe " + version.Version + "\n"
	want(t, "version", e.run(t, "", "version"), 0, v, "")
	want(t, "--version", e.run(t, "", "--version"), 0, v, "")
	if r := e.run(t, "", "--help"); r.code != 0 || !strings.Contains(r.stdout, "publish") {
		t.Errorf("--help = %+v", r)
	}
	want(t, "unknown command", e.run(t, "", "nope"), 2, "", `floe: unknown command "nope"`)
	want(t, "unknown flag", e.run(t, "", "list", "--nope"), 2, "", "floe: unknown flag: --nope")
	want(t, "publish without a file", e.run(t, "", "publish"), 2, "", "floe: accepts 1 arg(s), received 0")
	want(t, "rm without a slug", e.run(t, "", "rm"), 2, "", "floe: requires at least 1 arg(s)")
	want(t, "an empty list", e.run(t, "", "list"), 0, "", "")
	want(t, "stop without a server", e.run(t, "", "stop"), 0, "server is not running\n", "")
}

func TestConfigErrors(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	for _, tt := range []struct{ key, value, msg string }{
		{"FLOE_PORT", "abc", `floe: invalid FLOE_PORT "abc": use a port number from 1 to 65535`},
		{"FLOE_PORT", "0", `invalid FLOE_PORT "0"`},
		{"FLOE_PORT", "70000", `invalid FLOE_PORT "70000"`},
		{"FLOE_PORT", "0x10", `invalid FLOE_PORT "0x10"`},
		{"FLOE_PORT", "1e3", `invalid FLOE_PORT "1e3"`},
	} {
		saved := e.vars[tt.key]
		e.vars[tt.key] = tt.value
		want(t, tt.key+"="+tt.value, e.run(t, "", "list"), 1, "", tt.msg)
		e.vars[tt.key] = saved
	}
	delete(e.vars, "FLOE_DIR")
	delete(e.vars, "HOME")
	want(t, "no HOME", e.run(t, "", "list"), 1, "", "floe: HOME is not set; set HOME or FLOE_DIR")
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["LANG"] = "ja_JP.UTF-8"
	want(t, "stop in Japanese", e.run(t, "", "stop"), 0, "サーバーは動いていません\n", "")
	e.vars["FLOE_PORT"] = "abc"
	want(t, "a refusal in Japanese", e.run(t, "", "list"), 1, "",
		"floe: FLOE_PORT「abc」は使えません。1 から 65535 のポート番号を指定してください")
	want(t, "cobra's own errors stay in English", e.run(t, "", "list", "--nope"), 2, "", "floe: unknown flag: --nope")
	e.vars["FLOE_LANG"] = "en"
	want(t, "FLOE_LANG wins over LANG", e.run(t, "", "list"), 1, "", `floe: invalid FLOE_PORT "abc"`)
}

func TestPublish(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	demo := e.page(t, "My Demo.yaml", "Demo Page")
	url := e.base() + "/a/my-demo/"
	want(t, "publish", e.run(t, "repo", "publish", demo, "--description", "a demo"), 0, url+"\n", "")
	if got := e.opened(t); got != url+"\n" {
		t.Errorf("a first publish opened %q", got)
	}
	if r := e.run(t, "", "list"); !strings.HasSuffix(r.stdout, "\tmy-demo\tDemo Page\t\n") {
		t.Errorf("list = %q", r.stdout)
	}

	r := e.run(t, "repo", "publish", demo, "--slug", "my-demo")
	if r.code != 0 || r.stdout != url+"\n" || !strings.Contains(r.stderr, `updated "my-demo" (last published `) ||
		!strings.Contains(r.stderr, `, title "Demo Page")`) || strings.Contains(r.stderr, "warning") {
		t.Errorf("a republish = %+v", r)
	}
	r = e.run(t, "repo", "publish", demo, "--title", "Renamed")
	if !strings.Contains(r.stderr, "warning: the title changed; if this is a different page, publish it under another --slug") {
		t.Errorf("a changed title = %+v", r)
	}
	time.Sleep(300 * time.Millisecond)
	if got := e.opened(t); got != url+"\n" {
		t.Errorf("republishes opened more tabs: %q", got)
	}

	before := e.run(t, "", "list").stdout
	want(t, "a blank --title", e.run(t, "repo", "publish", demo, "--title", " "), 2, "",
		"floe: --title is empty: give the page a name, or omit --title to take the data file's title:, or a link's <title> or <h1>")
	if after := e.run(t, "", "list").stdout; after != before {
		t.Errorf("a blank --title changed the list from %q to %q", before, after)
	}
	want(t, "an invalid slug", e.run(t, "repo", "publish", demo, "--slug", "../x"), 1, "", `invalid slug "../x"`)
	want(t, "an invalid language", e.run(t, "repo", "publish", demo, "--lang", "ja_JP"), 2, "",
		`floe: invalid language "ja_JP": give --lang a BCP 47 tag such as en, ja or pt-BR`)
	frag := e.page(t, "frag.yaml", "こんにちは")
	want(t, "--lang", e.run(t, "repo", "publish", frag, "--lang", "ja"), 0, e.base()+"/a/frag/\n", "")
	if b, err := os.ReadFile(filepath.Join(e.vars["FLOE_DIR"], "frag", "index.html")); err != nil || !bytes.Contains(b, []byte(`<html lang="ja">`)) {
		t.Errorf("publish --lang ja did not draw the page in Japanese: %v", err)
	}
	want(t, "a missing file", e.run(t, "repo", "publish", "nope.yaml"), 1, "", "floe: cannot read nope.yaml")
	want(t, "another project", e.run(t, "other", "publish", demo), 1, "",
		`"my-demo" was published from project repo; use another --slug, or add --force to replace that page`)

	// A link: the same slug for another file is refused unless --force.
	site := e.write(t, "site/index.html", "<h1>Linked</h1>")
	want(t, "--lang with --link", e.run(t, "", "publish", site, "--link", "--lang", "ja"), 1, "",
		"floe: --lang is the language of a page drawn from a data file: a linked file is shown as it is")
	want(t, "publish --link", e.run(t, "", "publish", site, "--link"), 0, e.base()+"/a/site/\n", "")
	want(t, "re-register", e.run(t, "", "publish", site, "--link"), 0, e.base()+"/a/site/\n", `updated "site" (last published `)
	sub := e.write(t, "site/sub/p.html", "<title>Sub</title>")
	want(t, "another file", e.run(t, "", "publish", sub, "--link", "--slug", "site"), 1, "",
		`slug "site" is linked to `+site+"; choose another slug, or replace that link with floe publish --link --force")
	want(t, "--force", e.run(t, "", "publish", sub, "--link", "--slug", "site", "--force"), 0, e.base()+"/a/site/\n", "updated")
	if r := e.run(t, "", "list"); !strings.Contains(r.stdout, "\tsite\tSub\t"+sub+"\n") {
		t.Errorf("list after --force = %q", r.stdout)
	}

	want(t, "rm", e.run(t, "", "rm", "site", "my-demo", "frag", "nope", "untouched"), 1, "", `floe: no page "nope" (see floe list)`)
	want(t, "list after rm", e.run(t, "", "list"), 0, "", "")
	if _, err := os.Stat(sub); err != nil {
		t.Errorf("rm removed the linked file: %v", err)
	}

	h, _ := e.health(t)
	want(t, "stop", e.run(t, "", "stop"), 0, "stopped server (pid "+strconv.Itoa(h.PID)+")\n", "")
	if !e.wait(t.Context(), t, false) {
		t.Error("stop did not shut the server down")
	}
}

// TestPublishToASilentPort holds the port with a program that accepts connections and never answers:
// the publish gives up within the 3 s the server has to start, not after 150 probe timeouts.
func TestPublishToASilentPort(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	ln := e.hold(t)
	go func() {
		var held []net.Conn
		for {
			conn, err := ln.Accept()
			if err != nil {
				for _, c := range held {
					_ = c.Close()
				}
				return
			}
			held = append(held, conn)
		}
	}()
	start := time.Now()
	want(t, "publish", e.run(t, "", "publish", e.page(t, "silent.yaml", "Silent")), 1, "",
		`floe: the server did not start; run "floe serve" to see the error`)
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("the publish took %v", took)
	}
}

func TestPublishToAnotherServer(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	demo := e.page(t, "My Demo.yaml", "Demo")
	want(t, "publish", e.run(t, "", "publish", demo), 0, e.base()+"/a/my-demo/\n", "")
	e.vars["FLOE_DIR"] = filepath.Join(e.tmp, "other")
	want(t, "another store", e.run(t, "", "publish", demo), 1, "", "floe: the server on port "+e.vars["FLOE_PORT"]+
		" serves "+filepath.Join(e.tmp, "store")+`; run "floe stop" and publish again`)
	if _, err := os.Stat(filepath.Join(e.tmp, "other", "my-demo")); err == nil {
		t.Error("a failed publish left the page in the store")
	}

	o := newEnv(t)
	ln := o.hold(t)
	srv := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }() //nolint:errcheck // closed by the cleanup
	t.Cleanup(func() { _ = srv.Close() })
	want(t, "another program", o.run(t, "", "publish", demo), 1, "",
		"floe: port "+o.vars["FLOE_PORT"]+" is used by another program; set FLOE_PORT to a free port")
	want(t, "serve on a busy port", o.run(t, "", "serve"), 1, "",
		"floe: port "+o.vars["FLOE_PORT"]+" is in use; stop that program or set FLOE_PORT")
}

func TestServeInProcess(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, []string{"serve"}, cli.Env{
			Getenv: func(k string) string { return e.vars[k] }, Dir: e.tmp, Stdout: &stdout, Stderr: &stderr,
			Environ: nil,
		})
	}()
	// A serve that stops at once (its port taken) says why, rather than leave the wait to run out.
	upOrDone := make(chan bool, 1)
	go func() { upOrDone <- e.wait(ctx, t, true) }()
	select {
	case code := <-done:
		t.Fatalf("serve = %d before it came up: %s", code, stderr.String())
	case up := <-upOrDone:
		if !up {
			t.Fatal("serve did not come up")
		}
	}
	cancel()
	if code := <-done; code != 0 || stdout.String() != "serving "+e.vars["FLOE_DIR"]+" at "+e.base()+"/\n" {
		t.Errorf("serve = %d %q", code, stdout.String())
	}
}

func TestRestartOnAnotherBuild(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	page := e.page(t, "restart.yaml", "Restart")
	want(t, "publish", e.run(t, "", "publish", page), 0, e.base()+"/a/restart/\n", "")
	first, _ := e.health(t)
	bin := otherBuild
	want(t, "publish from another build", e.exe(t, bin, "publish", page), 0, e.base()+"/a/restart/\n", "updated")
	second, _ := e.health(t)
	want(t, "again", e.exe(t, bin, "publish", page), 0, e.base()+"/a/restart/\n", "updated")
	third, _ := e.health(t)
	if first.PID == second.PID || second.PID != third.PID || second.Build == first.Build {
		t.Errorf("pids %d %d %d: the server should restart once for the new build", first.PID, second.PID, third.PID)
	}

	// Two publishes racing to replace the server both succeed on the one that comes up.
	want(t, "back to this build", e.run(t, "", "publish", page), 0, e.base()+"/a/restart/\n", "updated")
	var wg sync.WaitGroup
	results := make([]result, 2)
	for i := range results {
		wg.Go(func() { results[i] = e.exe(t, bin, "publish", page) })
	}
	wg.Wait()
	for _, r := range results {
		if r.code != 0 {
			t.Errorf("a racing publish = %+v", r)
		}
	}
	if h, _ := e.health(t); h.Build != second.Build {
		t.Errorf("the server after the race runs %s, want %s", h.Build, second.Build)
	}
}

// A data file is published as a page drawn with its template, and a mistake in it is refused.
func TestPublishDataFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	plan := e.write(t, "plan.yaml", "template: report\ntitle: The plan\nsections:\n  - parts:\n      - text: Do it.\n")
	want(t, "publish a data file", e.run(t, "", "publish", plan), 0, e.base()+"/a/plan/\n", "")
	// Waiting for the browser to open the page also keeps it from writing after the test.
	if got := e.opened(t); got != e.base()+"/a/plan/\n" {
		t.Errorf("a first publish opened %q", got)
	}
	if b, err := os.ReadFile(filepath.Join(e.vars["FLOE_DIR"], "plan", "index.html")); err != nil || !bytes.Contains(b, []byte("<h1>The plan</h1>")) {
		t.Errorf("the published page = %s, %v", b, err)
	}
	e.vars["FLOE_LANG"] = "ja"
	bad := e.write(t, "bad.yaml", "title: t\n")
	want(t, "publish a data file without a template", e.run(t, "", "publish", bad), 1, "",
		bad+":1:1: 最上位に template: report のようにテンプレートを指定してください（テンプレート: report）")
	want(t, "publish --theme of no theme", e.run(t, "", "publish", plan, "--theme", "print"), 1, "",
		"floe: テンプレート report にテーマ「print」はありません。使えるのは default です")
	want(t, "publish --theme empty", e.run(t, "", "publish", plan, "--theme", " "), 2, "", "floe: --theme が空です")
}

// floe check reports the mistakes of every file at once, without publishing or starting the server,
// and prints nothing for files that are right.
func TestCheck(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.write(t, "ok.yaml", "template: report\ntitle: OK\n")
	e.write(t, "bad.yaml", "template: report\nleed: x\n")
	e.write(t, "bad.yml", "template: report\ntheme: print\n")
	want(t, "check a right file", e.run(t, "", "check", "ok.yaml"), 0, "", "")
	want(t, "check files with mistakes", e.run(t, "", "check", "ok.yaml", "bad.yaml", "bad.yml"), 1, "",
		"bad.yaml:2:1: leed is not an item of the template report (a typo?); use eyebrow, lead, sections, template, theme, title\n"+
			`bad.yml:2:8: template report has no theme "print"; use default`)
	// Mistakes at lines and columns go out as they are, for editors and CI to read.
	if r := e.run(t, "", "check", "bad.yaml"); !strings.HasPrefix(r.stderr, "bad.yaml:2:1: ") {
		t.Errorf("check prefixed its mistakes: %q", r.stderr)
	}
	// A file that cannot be checked is reported with the others, not instead of them.
	want(t, "check files that cannot be checked", e.run(t, "", "check", "page.html", "bad.yaml", "nope.yaml"), 1, "",
		"page.html is not a data file (.yaml or .yml)\nbad.yaml:2:1: leed is not an item")
	if r := e.run(t, "", "check", "bad.yaml", "nope.yaml"); !strings.Contains(r.stderr, "\ncannot read nope.yaml") {
		t.Errorf("check of a missing file after one with mistakes = %q", r.stderr)
	}
	want(t, "check without a file", e.run(t, "", "check"), 2, "", "floe: requires at least 1 arg(s)")
	if _, err := os.Stat(e.vars["FLOE_DIR"]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("check wrote to the store: %v", err)
	}
}
