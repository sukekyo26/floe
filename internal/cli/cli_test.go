package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	if err := os.Setenv(asCLI, "1"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// env is a store, a home and a free port in a temporary directory, with a browser that records what
// it is asked to open.
type env struct {
	tmp  string
	vars map[string]string
}

func freePort(t *testing.T) int {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || ln.Close() != nil {
		t.Fatal("no free port")
	}
	return addr.Port
}

func newEnv(t *testing.T) *env {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &env{tmp: tmp, vars: map[string]string{
		"HOME":      filepath.Join(tmp, "home"),
		"FLOE_DIR":  filepath.Join(tmp, "store"),
		"FLOE_PORT": strconv.Itoa(freePort(t)),
		"BROWSER":   filepath.Join(tmp, "browser"),
	}}
	e.write(t, "browser", "#!/bin/sh\nprintf '%s\\n' \"$1\" >>'"+filepath.Join(tmp, "opened")+"'\n")
	if err := os.Chmod(filepath.Join(tmp, "browser"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.vars["HOME"], 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.run(t, "", "stop") })
	return e
}

func (e *env) base() string {
	return "http://127.0.0.1:" + e.vars["FLOE_PORT"]
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

	cwd := filepath.Join(e.tmp, dir)
	if err := os.MkdirAll(cwd, 0o750); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(t.Context(), args, cli.Env{
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

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.base()+"/api/health", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return server.Health{}, false
	}
	defer res.Body.Close()
	var h server.Health
	return h, json.NewDecoder(res.Body).Decode(&h) == nil
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
		{"FLOE_IDLE_MINUTES", "0", `floe: invalid FLOE_IDLE_MINUTES "0": use a positive number of minutes`},
		{"FLOE_IDLE_MINUTES", "-1", `invalid FLOE_IDLE_MINUTES "-1"`},
		{"FLOE_IDLE_MINUTES", "1e3", `invalid FLOE_IDLE_MINUTES "1e3"`},
		{"FLOE_IDLE_MINUTES", "Inf", `invalid FLOE_IDLE_MINUTES "Inf"`},
		{"FLOE_IDLE_MINUTES", "NaN", `invalid FLOE_IDLE_MINUTES "NaN"`},
	} {
		saved := e.vars[tt.key]
		e.vars[tt.key] = tt.value
		want(t, tt.key+"="+tt.value, e.run(t, "", "list"), 1, "", tt.msg)
		e.vars[tt.key] = saved
	}
	e.vars["FLOE_IDLE_MINUTES"] = "0.01"
	want(t, "a fractional idle time", e.run(t, "", "list"), 0, "", "")
	delete(e.vars, "FLOE_DIR")
	delete(e.vars, "HOME")
	want(t, "no HOME", e.run(t, "", "list"), 1, "", "floe: HOME is not set; set HOME or FLOE_DIR")
}

func TestPublish(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	demo := e.write(t, "My Demo.html", "<title>Demo  Page</title><p>hi</p>\n")
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

	want(t, "a blank --title", e.run(t, "repo", "publish", demo, "--title", " "), 2, "",
		"floe: --title is empty: give the page a name, or omit --title to take it from the <title> or first <h1>")
	want(t, "an invalid slug", e.run(t, "repo", "publish", demo, "--slug", "../x"), 1, "", `invalid slug "../x"`)
	want(t, "a missing file", e.run(t, "repo", "publish", "nope.html"), 1, "", "floe: cannot read nope.html")
	want(t, "another project", e.run(t, "other", "publish", demo), 1, "",
		`"my-demo" was published from project repo; use another --slug, or add --force to replace that page`)

	// A link: the same slug for another file is refused unless --force.
	site := e.write(t, "site/index.html", "<h1>Linked</h1>")
	want(t, "publish --link", e.run(t, "", "publish", site, "--link"), 0, e.base()+"/a/site/\n", "")
	want(t, "re-register", e.run(t, "", "publish", site, "--link"), 0, e.base()+"/a/site/\n", `updated "site" (last published `)
	sub := e.write(t, "site/sub/p.html", "<title>Sub</title>")
	want(t, "another file", e.run(t, "", "publish", sub, "--link", "--slug", "site"), 1, "",
		`"site" is linked to `+site+"; use another --slug, or add --force to replace that link")
	want(t, "--force", e.run(t, "", "publish", sub, "--link", "--slug", "site", "--force"), 0, e.base()+"/a/site/\n", "updated")
	if r := e.run(t, "", "list"); !strings.Contains(r.stdout, "\tsite\tSub\t"+sub+"\n") {
		t.Errorf("list after --force = %q", r.stdout)
	}

	want(t, "rm", e.run(t, "", "rm", "site", "my-demo", "nope", "untouched"), 1, "", `floe: no page "nope" (see: floe list)`)
	want(t, "list after rm", e.run(t, "", "list"), 0, "", "")
	if _, err := os.Stat(sub); err != nil {
		t.Errorf("rm removed the linked file: %v", err)
	}

	h, up := e.health(t)
	want(t, "stop", e.run(t, "", "stop"), 0, "stopped server (pid "+strconv.Itoa(h.PID)+")\n", "")
	for range 50 {
		if _, up = e.health(t); !up {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if up {
		t.Error("stop did not shut the server down")
	}
}

func TestPublishToAnotherServer(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	demo := e.write(t, "My Demo.html", "<title>Demo</title>")
	want(t, "publish", e.run(t, "", "publish", demo), 0, e.base()+"/a/my-demo/\n", "")
	e.vars["FLOE_DIR"] = filepath.Join(e.tmp, "other")
	want(t, "another store", e.run(t, "", "publish", demo), 1, "", "floe: the server on port "+e.vars["FLOE_PORT"]+
		" serves "+filepath.Join(e.tmp, "store")+`; run "floe stop" and publish again`)
	if _, err := os.Stat(filepath.Join(e.tmp, "other", "my-demo")); err == nil {
		t.Error("a failed publish left the page in the store")
	}

	o := newEnv(t)
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:"+o.vars["FLOE_PORT"])
	if err != nil {
		t.Fatal(err)
	}
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
	var stdout bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, []string{"serve"}, cli.Env{
			Getenv: func(k string) string { return e.vars[k] }, Dir: e.tmp, Stdout: &stdout, Stderr: io.Discard,
			Environ: nil,
		})
	}()
	up := false
	for range 100 {
		if _, up = e.health(t); up {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !up {
		t.Fatal("serve did not come up")
	}
	cancel()
	if code := <-done; code != 0 || stdout.String() != "serving "+e.vars["FLOE_DIR"]+" at "+e.base()+"/\n" {
		t.Errorf("serve = %d %q", code, stdout.String())
	}
}

// copyBinary copies this test binary, as a reinstalled floe with another build.
func copyBinary(t *testing.T, e *env) string {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(e.tmp, "floe-copy")
	if err := os.WriteFile(bin, append(b, 0), 0o700); err != nil { //nolint:gosec // G306: an executable
		t.Fatal(err)
	}
	return bin
}

func TestRestartOnAnotherBuild(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	page := e.write(t, "restart.html", "<h1>Restart</h1>")
	want(t, "publish", e.run(t, "", "publish", page), 0, e.base()+"/a/restart/\n", "")
	first, _ := e.health(t)
	bin := copyBinary(t, e)
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

func TestIdleExit(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_IDLE_MINUTES"] = "0.01"
	want(t, "publish", e.run(t, "", "publish", e.write(t, "idle.html", "<h1>Idle</h1>")), 0, e.base()+"/a/idle/\n", "")
	// Each probe is a request that restarts the 0.6 s timer, so probes are a second apart.
	up := true
	for range 4 {
		time.Sleep(time.Second)
		if _, up = e.health(t); !up {
			break
		}
	}
	if up {
		t.Error("the server did not exit after the idle time")
	}
}
