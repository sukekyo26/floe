package cli_test

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
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

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/server"
)

// asCLI makes the test binary run as floe, so that the server a publish starts (os.Executable() serve) is this
// binary.
const asCLI = "FLOE_TEST_AS_CLI"

func TestMain(m *testing.M) {
	if os.Getenv(asCLI) == "1" {
		dir, _ := os.Getwd() //nolint:errcheck // as main does
		os.Exit(cli.Run(os.Args[1:], cli.Env{Stdout: os.Stdout, Stderr: os.Stderr, Dir: dir, Getenv: os.Getenv}))
	}
	if err := os.Setenv(asCLI, "1"); err != nil {
		panic(err)
	}
	var err error
	if shared, err = os.MkdirTemp("", "floe-cli-test-"); err != nil {
		panic(err)
	}
	if otherBuild, err = copySelf(shared); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(shared) //nolint:errcheck // a leftover temporary directory is harmless
	os.Exit(code)
}

// shared is a temporary directory of the whole run.
var shared string

// otherBuild is a copy of this test binary, as a reinstalled floe with another build.
var otherBuild string

// copySelf writes otherBuild. It runs before the parallel tests: a process they fork while the copy is open for
// writing would hold it, and running the copy would fail with ETXTBSY (golang/go#22315).
func copySelf(dir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err //nolint:wrapcheck // TestMain panics with it
	}
	b, err := os.ReadFile(self)
	if err != nil {
		return "", err //nolint:wrapcheck // as above
	}
	bin := filepath.Join(dir, "floe-copy")
	//nolint:gosec // G306: an executable
	return bin, os.WriteFile(bin, append(b, 0), 0o700) //nolint:wrapcheck // as above
}

// freePort returns a port free now and held for this test alone on this machine, below the ports the system gives
// out for :0, for a server a publish starts on it later: a lock of the port's file, which another test, in this
// process or another (a just ci of another worktree), cannot take until this one ends.
// LIMIT: a system set to give out ports from below 32768 may still give it; read its range if a CI comes to.
// LIMIT: a program other than these tests may take the port between its check here and the publish's bind, or while
// TestRestartOnAnotherBuild replaces the server; take the port from the publish if that comes to happen.
// LIMIT: what took a port of these tests on a CI runner once (a run of 2026-10-27, which runs one process of them) is
// not known; narrow it down by the line and the port if it fails again.
// LIMIT: tests of another TMPDIR do not see the locks; give them one if two such runs share a machine.
func freePort(t *testing.T) int {
	t.Helper()

	return pick(t, 20000, 32767)
}

// pick holds a port from lo to hi (both in) that no other test holds and nothing listens on.
func pick(t *testing.T, lo, hi int) int {
	t.Helper()

	for range 100 {
		if port := lo + rand.IntN(hi-lo+1); hold(t, port) { //nolint:gosec // G404: a port to try, no secret
			return port
		}
	}
	t.Fatal("no free port")
	return 0
}

// hold locks port for t until it ends, when no other test holds it and nothing listens on it.
func hold(t *testing.T, port int) bool {
	t.Helper()

	lock, err := os.OpenFile(filepath.Join(portLocks(t), strconv.Itoa(port)), os.O_RDONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: the tests' own
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G115: a descriptor fits an int
	if unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil || !free(t, port) {
		_ = lock.Close() //nolint:errcheck // nothing written
		return false
	}
	t.Cleanup(func() { _ = lock.Close() }) //nolint:errcheck // as above; closing it lets the lock go
	return true
}

// portLocks is the directory of the locks of the ports, one per user: another's, made 0o700, could not be opened.
// A lock's file stays: removing it would let another process lock a new file of the same name beside it.
func portLocks(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "floe-test-ports-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// free is whether nothing listens on port now.
func free(t *testing.T, port int) bool {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	if ln.Close() != nil {
		t.Fatal("no free port")
	}
	return true
}

// denv is a store, a home and a free port in a temporary directory, with the server a publish starts there
// stopped when the test ends.
type denv struct {
	tmp  string
	vars map[string]string
	port string
}

func newDenv(t *testing.T) *denv {
	t.Helper()

	return newDenvOn(t, freePort(t))
}

// newDenvOn is newDenv on port, which the test holds.
func newDenvOn(t *testing.T, port int) *denv {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &denv{tmp: tmp, port: strconv.Itoa(port), vars: map[string]string{
		"HOME": filepath.Join(tmp, "home"), "FLOE_DIR": filepath.Join(tmp, "store"), "FLOE_PORT": strconv.Itoa(port), "BROWSER": "true",
	}}
	if err := os.MkdirAll(e.vars["HOME"], 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "page.yaml"), []byte("lang: en\ntemplate: report\ntitle: P\nsections:\n  - heading: h\n    parts:\n      - text: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.stop(t) })
	return e
}

func (e *denv) base() string { return "http://127.0.0.1:" + e.vars["FLOE_PORT"] }

type result struct {
	code           int
	stdout, stderr string
}

// run runs floe in process, from tmp.
func (e *denv) run(args ...string) result {
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: e.tmp, Getenv: func(k string) string { return e.vars[k] }})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// exe runs the binary bin as floe in a process of its own, with only the settings of e.
func (e *denv) exe(t *testing.T, bin string, args ...string) result {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), bin, args...)
	cmd.Dir = e.tmp
	cmd.Env = []string{asCLI + "=1", "PATH=" + os.Getenv("PATH")}
	for k, v := range e.vars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func (e *denv) health(ctx context.Context) (server.Health, bool) {
	p := daemon.Probe(ctx, e.base())
	return p.Health, p.State == daemon.Ours
}

// stop stops the server a test left on its port, and fails the test when the server outlives it. It runs as a
// cleanup, after t.Context() is canceled, so it takes a context that is not.
func (e *denv) stop(t *testing.T) {
	t.Helper()

	ctx := context.WithoutCancel(t.Context())
	e.vars["FLOE_PORT"] = e.port
	if h, up := e.health(ctx); up && h.PID != os.Getpid() {
		if r := e.run("stop"); r.code != 0 {
			t.Errorf("stop = %+v", r)
		}
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, up := e.health(ctx); !up {
			return
		}
		if time.Now().After(deadline) {
			t.Error("the server outlived the test")
			return
		}
	}
}

func wantRun(t *testing.T, what string, got result, code int, stdout, stderr string) {
	t.Helper()

	if got.code != code || got.stdout != stdout || !strings.Contains(got.stderr, stderr) || stderr == "" && got.stderr != "" {
		t.Errorf("%s = %d %q %q, want %d %q and stderr containing %q", what, got.code, got.stdout, got.stderr, code, stdout, stderr)
	}
}

// A port another test holds is not held again though nothing listens on it: in this process, as in another, whose
// lock of the port's file is the same.
func TestHoldSkipsHeld(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	if hold(t, port) {
		t.Errorf("port %d, held by this test, was held again", port)
	}
}

// A publish starts the server, one of another build replaces it once, and two racing to replace it both succeed.
func TestRestartOnAnotherBuild(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	url := e.base() + "/a/page/\n"
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, url, "")
	first, _ := e.health(t.Context())
	wantRun(t, "publish from another build", e.exe(t, otherBuild, "publish", "page.yaml"), 0, url, "updated")
	second, _ := e.health(t.Context())
	wantRun(t, "again", e.exe(t, otherBuild, "publish", "page.yaml"), 0, url, "updated")
	third, _ := e.health(t.Context())
	if first.PID == second.PID || second.PID != third.PID || second.Build == first.Build {
		t.Errorf("pids %d %d %d: the server should restart once for the new build", first.PID, second.PID, third.PID)
	}
	wantRun(t, "back to this build", e.run("publish", "page.yaml"), 0, url, "updated")
	var wg sync.WaitGroup
	results := make([]result, 2)
	for i := range results {
		wg.Go(func() { results[i] = e.exe(t, otherBuild, "publish", "page.yaml") })
	}
	wg.Wait()
	for _, r := range results {
		if r.code != 0 {
			t.Errorf("a racing publish = %+v", r)
		}
	}
}

// A publish that means other parts, themes or icons than the server's is refused, from another build too, which
// would replace the server; listing and removing draw nothing and take it as it is. With no place for the icons,
// the command and the server both mean none, and a publish again is no other.
func TestPublishOtherDirs(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, e.base()+"/a/page/\n", "")
	first, _ := e.health(t.Context())
	refused := "floe: the server on port " + e.port + " draws pages with the templates of " + first.Templates
	for key, dir := range map[string]string{"FLOE_ICONS_DIR": "other-icons", "FLOE_THEMES_DIR": "other-themes", "FLOE_PARTS_DIR": "other-parts"} {
		e.vars[key] = filepath.Join(e.tmp, dir)
		if err := os.Mkdir(e.vars[key], 0o700); err != nil {
			t.Fatal(err)
		}
		wantRun(t, "publish with "+dir, e.run("publish", "page.yaml"), 1, "", refused)
		wantRun(t, "publish with "+dir+" from another build", e.exe(t, otherBuild, "publish", "page.yaml"), 1, "", refused)
		if r := e.run("list"); r.code != 0 || !strings.Contains(r.stdout, "\tpage\t") {
			t.Errorf("list with %s = %+v", dir, r)
		}
		delete(e.vars, key)
	}
	if h, _ := e.health(t.Context()); h.PID != first.PID {
		t.Errorf("the server was replaced: pid %d, then %d", first.PID, h.PID)
	}

	n := newDenv(t)
	n.vars["HOME"] = "rel"
	wantRun(t, "publish with no place for the icons", n.run("publish", "page.yaml"), 0, n.base()+"/a/page/\n", "")
	if h, _ := n.health(t.Context()); h.Icons != "" {
		t.Errorf("the server's icons are %q, want none", h.Icons)
	}
	wantRun(t, "again", n.run("publish", "page.yaml"), 0, n.base()+"/a/page/\n", "updated")
}

// A publish to a server of another store, or to a port another program holds, is refused.
func TestPublishToAnotherServer(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, e.base()+"/a/page/\n", "")
	e.vars["FLOE_DIR"] = filepath.Join(e.tmp, "other")
	wantRun(t, "another store", e.run("publish", "page.yaml"), 1, "", "floe: the server on port "+e.port+
		" serves "+filepath.Join(e.tmp, "store")+`; run "floe stop" and publish again`)
	if _, err := os.Stat(filepath.Join(e.tmp, "other", "page")); err == nil {
		t.Error("a failed publish left the page in the store")
	}
	e.vars["FLOE_DIR"] = filepath.Join(e.tmp, "store")

	// Another program on a port: one the system picks and this test holds, so that no other can take it.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("a TCP listener on %v", ln.Addr())
	}
	o := newDenvOn(t, addr.Port)
	srv := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }() //nolint:errcheck // closed by the cleanup
	t.Cleanup(func() { _ = srv.Close() })
	wantRun(t, "another program", o.run("publish", "page.yaml"), 1, "",
		"floe: port "+o.port+" is used by another program; set FLOE_PORT to a free port")
	wantRun(t, "serve on a busy port", o.run("serve"), 1, "",
		"floe: port "+o.port+" is in use; stop that program (floe stop, if it is floe's server) or set FLOE_PORT")
}
