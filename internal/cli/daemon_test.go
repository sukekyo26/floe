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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"floe/internal/cli"
	"floe/internal/daemon"
	"floe/internal/server"
	"floe/internal/servetest"
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

	for range 100 {
		if port := 20000 + rand.IntN(12768); hold(t, port) { //nolint:gosec // G404: a port to try, no secret
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
	fd := int(lock.Fd()) //nolint:gosec // G115: a descriptor fits an int
	if unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB) != nil {
		_ = lock.Close() //nolint:errcheck // nothing written
		return false
	}
	// Closing it lets the lock go, however the test ends from here.
	t.Cleanup(func() { _ = lock.Close() }) //nolint:errcheck // as above
	if !free(t, port) {
		_ = lock.Close() //nolint:errcheck // as above; the cleanup's close then does nothing
		return false
	}
	return true
}

// portLocks is the directory of the locks of the ports, one per user: another's, made 0o700, could not be opened.
// A lock's file stays: removing it would let another process lock a new file of the same name beside it.
// LIMIT: up to 12768 empty files gather in it, one per port tried; remove the directory when no test runs if they come to matter.
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
	p := strconv.Itoa(port)
	e := &denv{tmp: tmp, port: p, vars: map[string]string{
		"HOME": filepath.Join(tmp, "home"), "FLOE_DIR": filepath.Join(tmp, "store"), "FLOE_PORT": p, "BROWSER": "true",
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

// projectLine is what floe publish says of a new page; its name is of a directory made for the test, which changes.
const projectLine = "page in project "

func wantRun(t *testing.T, what string, got result, code int, stdout, stderr string) {
	t.Helper()

	if got.code != code || got.stdout != stdout || !strings.Contains(got.stderr, stderr) || stderr == "" && got.stderr != "" {
		t.Errorf("%s = %d %q %q, want %d %q and stderr containing %q", what, got.code, got.stdout, got.stderr, code, stdout, stderr)
	}
}

// A port another test holds is not held again though nothing listens on it. The test holds it twice in one process;
// another process is refused alike, as flock locks a file opened, not a process.
func TestHoldSkipsHeld(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	if hold(t, port) {
		t.Errorf("port %d, held by this test, was held again", port)
	}
}

// A publish starts the server, one of another build replaces it once, and two racing to replace it both succeed. The
// command that stops the server of another build says so on stderr, once and only then; the log of the server says
// why it stopped before the next one says it serves, and a line it writes long after it started is there.
func TestRestartOnAnotherBuild(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	url := e.base() + "/a/page/\n"
	stopped := func(pid int) string {
		return "stopping the server on port " + e.port + " (pid " + strconv.Itoa(pid) + "), which was of another build of floe"
	}
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, url, projectLine)
	first, _ := e.health(t.Context())
	r := e.exe(t, otherBuild, "publish", "page.yaml")
	wantRun(t, "publish from another build", r, 0, url, "updated")
	if !strings.Contains(r.stderr, stopped(first.PID)) {
		t.Errorf("the publish that replaced the server said %q, want %q", r.stderr, stopped(first.PID))
	}
	second, _ := e.health(t.Context())
	r = e.exe(t, otherBuild, "publish", "page.yaml")
	wantRun(t, "again", r, 0, url, "updated")
	if strings.Contains(r.stderr, "stopping the server") {
		t.Errorf("a publish of the build that runs the server said %q", r.stderr)
	}
	third, _ := e.health(t.Context())
	if first.PID == second.PID || second.PID != third.PID || second.Build == first.Build {
		t.Errorf("pids %d %d %d: the server should restart once for the new build", first.PID, second.PID, third.PID)
	}
	r = e.run("publish", "page.yaml")
	wantRun(t, "back to this build", r, 0, url, "updated")
	if !strings.Contains(r.stderr, stopped(second.PID)) {
		t.Errorf("the publish back said %q, want %q", r.stderr, stopped(second.PID))
	}
	// The first server wrote that it stops before the second says it serves, and wrote that it stopped after it was
	// asked, long after it started (the order of that line and the second's is not fixed).
	log := e.logWith(t, "msg=stopped", "pid="+strconv.Itoa(first.PID))
	stopping := lineAt(log, "msg=stopping", "pid="+strconv.Itoa(first.PID), `why="asked to stop"`)
	serving := lineAt(log, "msg=serving", "pid="+strconv.Itoa(second.PID))
	if stopping < 0 || serving < 0 || stopping > serving {
		t.Errorf("the log = %q: want the first server's stopping before the second's serving", log)
	}
	var wg sync.WaitGroup
	results := make([]result, 2)
	for i := range results {
		wg.Go(func() { results[i] = e.exe(t, otherBuild, "publish", "page.yaml") })
	}
	wg.Wait()
	// Of two commands asking the server to stop together, one is answered 204 and says so: the other is told it is
	// stopping already, and the server it stops is not its doing.
	said := 0
	for _, r := range results {
		if r.code != 0 {
			t.Errorf("a racing publish = %+v", r)
		}
		said += strings.Count(r.stderr, "stopping the server")
	}
	if said != 1 {
		t.Errorf("two racing commands said that they stopped the server %d times, want once: %q %q", said, results[0].stderr, results[1].stderr)
	}
}

// lineAt is the number of the first line of log that holds all of parts, each between spaces (or the line's ends), so
// that pid=4521 is not pid=45210; or -1.
func lineAt(log string, parts ...string) int {
	for i, line := range strings.Split(log, "\n") {
		padded := " " + line + " "
		if !slices.ContainsFunc(parts, func(p string) bool { return !strings.Contains(padded, " "+p+" ") }) {
			return i
		}
	}
	return -1
}

// lineAt takes a part between spaces or a line's ends: pid=4521 is not a part of a line with pid=45210.
func TestLineAt(t *testing.T) {
	t.Parallel()

	log := "time=t level=INFO msg=serving pid=45210 why=\"asked to stop\"\ntime=t msg=stopped pid=4521"
	for _, c := range []struct {
		parts []string
		want  int
	}{
		{[]string{"msg=serving", "pid=45210"}, 0},
		{[]string{"pid=4521"}, 1},
		{[]string{"msg=serving", "pid=4521"}, -1},
		{[]string{`why="asked to stop"`}, 0},
		{[]string{"pid=452"}, -1},
	} {
		if got := lineAt(log, c.parts...); got != c.want {
			t.Errorf("lineAt(%q) = %d, want %d", c.parts, got, c.want)
		}
	}
}

// logWith is the log of the server of e's store once it has a line that holds all of want (lineAt), which the server
// may write after the command that asked for it ended: waited for 5 s.
func (e *denv) logWith(t *testing.T, want ...string) string {
	t.Helper()

	path := filepath.Join(e.vars["FLOE_DIR"], server.LogName)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		b, _ := os.ReadFile(path) //nolint:errcheck // not there yet
		if lineAt(string(b), want...) >= 0 {
			return string(b)
		}
		if time.Now().After(deadline) {
			t.Errorf("%s = %q, want it to hold a line with %q", server.LogName, b, want)
			return string(b)
		}
	}
}

// A server of another build is stopped and none starts in its place (the store cannot be written: the new server's
// serve refuses to start), and the command says that it stopped the server before it says the server did not start.
func TestStoppedThenNotStarted(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root writes where it is told not to")
	}
	e := newDenv(t)
	wantRun(t, "publish from another build", e.exe(t, otherBuild, "publish", "page.yaml"), 0, e.base()+"/a/page/\n", projectLine)
	old, _ := e.health(t.Context())
	store := e.vars["FLOE_DIR"]
	if err := os.Chmod(store, 0o500); err != nil {
		t.Fatal(err)
	}
	// Before the cleanup that stops the server, which needs it writable to be asked.
	t.Cleanup(func() { _ = os.Chmod(store, 0o700) }) //nolint:errcheck // a temporary directory

	r := e.run("work", "--project", "p", "--task", "t1", "--title", "x")
	said := "stopping the server on port " + e.port + " (pid " + strconv.Itoa(old.PID) + "), which was of another build of floe"
	didNot := "the server did not start"
	i, j := strings.Index(r.stderr, said), strings.Index(r.stderr, didNot)
	if r.code != 1 || i < 0 || j < i {
		t.Errorf("work after the server of another build was stopped and none started = %d %q, want 1 and %q, then %q",
			r.code, r.stderr, said, didNot)
	}
}

// A server that is made to crash (SIGQUIT) leaves what the runtime wrote of it in server.log: the standard error of
// the server a command starts is discarded, so the log is where it is.
func TestCrashOutput(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, e.base()+"/a/page/\n", projectLine)
	h, up := e.health(t.Context())
	if !up {
		t.Fatal("no server after a publish")
	}
	if err := unix.Kill(h.PID, unix.SIGQUIT); err != nil {
		t.Fatal(err)
	}
	log := e.logWith(t, "goroutine")
	if !strings.Contains(log, "SIGQUIT") {
		t.Errorf("%s = %q, want the crash in it", server.LogName, log)
	}
}

// A publish or a rebuild that means other parts, themes or icons than the server's is refused, from another build
// too, which would replace the server, and told to run the command again; listing and removing draw nothing and
// take it as it is. With no place for the icons,
// the command and the server both mean none, and a second publish is no other.
func TestPublishOtherDirs(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, e.base()+"/a/page/\n", projectLine)
	first, _ := e.health(t.Context())
	refused := "floe: the server on port " + e.port + " draws pages with the templates of " + strconv.Quote(first.Templates)
	again := `; move yours where the server reads them, or run "floe stop" and run this command again` + "\n"
	// Names with a space and a semicolon: the message must show where a path ends.
	for key, dir := range map[string]string{"FLOE_ICONS_DIR": "other icons; 1", "FLOE_THEMES_DIR": "other themes; 1", "FLOE_PARTS_DIR": "other parts; 1"} {
		e.vars[key] = filepath.Join(e.tmp, dir)
		if err := os.Mkdir(e.vars[key], 0o700); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"publish", "page.yaml"}, {"rebuild"}} {
			r := e.run(args...)
			wantRun(t, args[0]+" with "+dir, r, 1, "", refused)
			if !strings.HasSuffix(r.stderr, again) {
				t.Errorf("%s with %s = %q, want it to end in %q", args[0], dir, r.stderr, again)
			}
			if !slices.Contains(quotedPaths(r.stderr), e.vars[key]) {
				t.Errorf("%s with %s = %q, want the directory this command means in quotes", args[0], dir, r.stderr)
			}
		}
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
	wantRun(t, "publish with no place for the icons", n.run("publish", "page.yaml"), 0, n.base()+"/a/page/\n", projectLine)
	if h, _ := n.health(t.Context()); h.Icons != "" {
		t.Errorf("the server's icons are %q, want none", h.Icons)
	}
	wantRun(t, "again", n.run("publish", "page.yaml"), 0, n.base()+"/a/page/\n", "updated")
}

var quotedRe = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// quotedPaths are the absolute paths a message writes in quotes, as strconv.Quote does, in order.
func quotedPaths(msg string) []string {
	var paths []string
	for _, q := range quotedRe.FindAllString(msg, -1) {
		if p, err := strconv.Unquote(q); err == nil && filepath.IsAbs(p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// A command to a server of another store, or to a port another program holds, is refused.
func TestPublishToAnotherServer(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	// A name with a space and a semicolon: the message must show where the path ends.
	store := filepath.Join(e.tmp, "my store; 1")
	e.vars["FLOE_DIR"] = store
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, e.base()+"/a/page/\n", projectLine)
	// A serve that loses the port to a floe server writes nothing in the store, the log over the cap too, which it would
	// move aside from under the server that holds the port if it opened the log before it had the port.
	held := filepath.Join(store, server.LogName)
	big, err := os.OpenFile(held, os.O_WRONLY|os.O_APPEND, 0) //nolint:gosec // G304: the test's own
	if err != nil {
		t.Fatal(err)
	}
	if _, err = big.Write(bytes.Repeat([]byte("x"), 2<<20)); err != nil {
		t.Fatal(err)
	}
	_ = big.Close() //nolint:errcheck // written
	wantRun(t, "serve on a port a floe server holds", e.run("serve"), 1, "", "is in use")
	if _, err = os.Stat(held + ".1"); err == nil {
		t.Error("a floe that lost the port moved the log of the server that holds it aside")
	}
	other := filepath.Join(e.tmp, "other")
	for _, args := range [][]string{{"publish", "page.yaml"}, {"rebuild"}, {"list"}} {
		e.vars["FLOE_DIR"] = other
		r := e.run(args...)
		if r.code != 1 || r.stdout != "" || !strings.HasPrefix(r.stderr, "floe: ") || !strings.Contains(r.stderr, "port "+e.port) ||
			!strings.Contains(r.stderr, "FLOE_DIR") || !strings.Contains(r.stderr, "FLOE_PORT") || !strings.Contains(r.stderr, "floe stop") {
			t.Errorf("%s to another store = %+v, want a refusal that names port %s, FLOE_DIR, FLOE_PORT and floe stop", args[0], r, e.port)
			continue
		}
		// The store the server serves, said twice: where it serves, and what FLOE_DIR is to be set to.
		told := quotedPaths(r.stderr)
		if len(told) != 2 || told[0] != told[1] {
			t.Errorf("%s to another store names the paths %q, want the server's store twice", args[0], told)
			continue
		}
		// Taken from the message: set so, the command is not refused for its store.
		e.vars["FLOE_DIR"] = told[0]
		if r = e.run(args...); r.code != 0 {
			t.Errorf("%s with FLOE_DIR set as the refusal says = %+v, want it to go through", args[0], r)
		}
	}
	if _, err := os.Stat(filepath.Join(other, "page")); err == nil {
		t.Error("a failed publish left the page in the store")
	}
	e.vars["FLOE_DIR"] = store

	// Another program on a port.
	ln, port := holdPort(t)
	o := newDenvOn(t, port)
	srv := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }() //nolint:errcheck // closed by the cleanup
	t.Cleanup(func() { _ = srv.Close() })
	wantRun(t, "another program", o.run("publish", "page.yaml"), 1, "",
		"floe: port "+o.port+" is used by another program; set FLOE_PORT to a free port")
	wantRun(t, "serve on a busy port", o.run("serve"), 1, "",
		"floe: port "+o.port+" is in use; stop that program (floe stop, if it is floe's server) or set FLOE_PORT")
	// It took no port: it writes nothing of what a server keeps.
	for _, name := range []string{"reads.json", server.LogName} {
		if _, err := os.Stat(filepath.Join(o.vars["FLOE_DIR"], name)); err == nil {
			t.Errorf("a floe that cannot serve wrote %s", name)
		}
	}
}

// holdPort listens on a port the system picks, held until the test ends so that no other can take it, and
// answers nothing until the test serves it.
func holdPort(t *testing.T) (net.Listener, int) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() }) //nolint:errcheck // closed by a server's Close too, when one serves it
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("a TCP listener on %v", ln.Addr())
	}
	return ln, addr.Port
}

// floe stop returns once the server no longer answers, so that a command right after starts its own.
func TestStopWaits(t *testing.T) {
	t.Parallel()

	ln, port := holdPort(t)
	e := newDenvOn(t, port)
	servetest.Fake{PID: 4242, Linger: 300 * time.Millisecond}.Serve(t, ln) //nolint:exhaustruct // the rest unset
	wantRun(t, "stop", e.run("stop"), 0, "stopped server (pid 4242)\n", "")
	if _, up := e.health(t.Context()); up {
		t.Error("the server still answers after floe stop")
	}
}

// A server gone before it is asked, another serving the port by then, is told so: floe stop does not stop that one.
func TestStopAnother(t *testing.T) {
	t.Parallel()

	ln, port := holdPort(t)
	e := newDenvOn(t, port)
	servetest.Fake{PID: 4242, Next: 4343, Keeps: true}.Serve(t, ln) //nolint:exhaustruct // the rest unset
	wantRun(t, "stop", e.run("stop"), 0, "server (pid 4242) is gone, and another (pid 4343) serves the port now\n", "")
}

// A server that does not take the request (of a build without it) or answers on after it is told with its port and
// pid, and floe stop fails; why a request to stop failed follows, in net/http's words.
func TestStopKeptAnswering(t *testing.T) {
	t.Parallel()

	still := "is still running; stop it yourself (the process 4242, if it runs on this machine), and the next floe" +
		" command that uses the server starts its own\n"
	for _, c := range []struct {
		name string
		fake servetest.Fake
		// at once: one that does not take the request is not waited for, which takes 10 s.
		atOnce bool
		want   func(port string) string
	}{
		{"no request", servetest.Fake{PID: 4242, Answer: http.StatusNotFound, Keeps: true}, true, //nolint:exhaustruct // the rest unset
			func(port string) string { return "floe: the server on port " + port + " " + still }},
		{"answers on", servetest.Fake{PID: 4242, Keeps: true}, false, //nolint:exhaustruct // the rest unset
			func(port string) string { return "floe: the server on port " + port + " " + still }},
		{"no answer", servetest.Fake{PID: 4242, Hang: true, Keeps: true}, false, //nolint:exhaustruct // the rest unset
			func(port string) string {
				return "floe: the server on port " + port + " " + still +
					`floe: Post "http://127.0.0.1:` + port + `/api/stop": context deadline exceeded` + "\n"
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ln, port := holdPort(t)
			e := newDenvOn(t, port)
			c.fake.Serve(t, ln)
			start := time.Now()
			wantRun(t, "stop", e.run("stop"), 1, "", c.want(e.port))
			if took := time.Since(start); c.atOnce && took > time.Second {
				t.Errorf("floe stop took %v, waiting for a server that did not take the request", took)
			}
		})
	}
}

// floe stop signals no process: one whose pid a server tells goes on running.
func TestStopSendsNoSignal(t *testing.T) {
	t.Parallel()

	child := exec.CommandContext(context.WithoutCancel(t.Context()), "sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	// Reaped as it ends, so that one ended reads as gone, not as a zombie.
	ended := make(chan struct{})
	go func() {
		_ = child.Wait() //nolint:errcheck // ended either way
		close(ended)
	}()
	t.Cleanup(func() { _ = child.Process.Kill() }) //nolint:errcheck // gone already, when stopped
	ln, port := holdPort(t)
	e := newDenvOn(t, port)
	//nolint:exhaustruct // the rest unset
	servetest.Fake{PID: child.Process.Pid, Answer: http.StatusNotFound, Keeps: true}.Serve(t, ln)
	if r := e.run("stop"); r.code != 1 {
		t.Errorf("stop = %+v", r)
	}
	select {
	case <-ended:
		t.Error("the process the server told was stopped")
	case <-time.After(100 * time.Millisecond):
	}
}

// A port held by a program that never answers is refused, and the command is told to run again. Apart from the
// others, as the probes and the waits take seconds.
func TestSilentPort(t *testing.T) {
	t.Parallel()

	_, port := holdPort(t)
	e := newDenvOn(t, port)
	wantRun(t, "list", e.run("list"), 1, "", "floe: port "+e.port+" is in use, and nothing answers there: a server"+
		" another floe command started may still be starting, or another program holds it; run this command again,"+
		" or set FLOE_PORT to a free port\n")
}
