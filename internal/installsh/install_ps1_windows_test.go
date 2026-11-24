package installsh_test

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// LIMIT: the real Invoke-WebRequest is run, but against a server of the test, through a proxy of the test, with its
// certificate trusted by the driver alone: GitHub itself and a certificate checked as it is are not. The branch that
// tells an arm64 machine under an x64 PowerShell is not taken on amd64 runners. That the change of PATH reaches other
// programs (the notice) is seen on a real machine alone.

// ghRelease is what the test's GitHub answers: the latest release's JSON, and the files of every tag.
type ghRelease struct {
	apiStatus int               // 0 is 200
	api       string            // the body of the API's answer
	files     map[string][]byte // the body of each file, any tag
	sums      string            // SHA256SUMS; "" makes it from files
	serve     map[string]func(w http.ResponseWriter, r *http.Request, host string)
	mute      <-chan struct{} // not nil: the proxy answers a CONNECT and goes no further, until it is closed
	hits      atomic.Int32    // requests the server answered
	plain     atomic.Int32    // of them, requests in plain http

	// lose names the CONNECTs the proxy loses, in order: one to lose[0] answers 200 and is never read from, until
	// the test ends, the stop Windows PowerShell 5.1 makes now and then, made. Closed at once instead, it is not:
	// Windows PowerShell opens another tunnel by itself and is answered (seen on CI, run 44757137359).
	lose    []string
	loseMu  sync.Mutex
	tunnels atomic.Int32 // CONNECTs tunnelled on to the TLS server, or lost
	spoken  atomic.Int32 // of them, ones a byte of the client's was read on
}

// firstRead calls seen once, at the first byte read.
type firstRead struct {
	r    io.Reader
	once sync.Once
	seen func()
}

func (f *firstRead) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if n > 0 {
		f.once.Do(f.seen)
	}
	return n, err
}

const (
	repoPath  = "/sukekyo26/floe/releases/download/"
	githubURL = "https://github.com" + repoPath
	// silentPath is where the proxy tells the driver how many tunnels it heard nothing on.
	silentPath = "/floe-test/silent"
)

var (
	drivesMu sync.Mutex
	drives   = map[string]bool{} // letters a running test has taken
)

// freeDrive is a letter no drive has and no other test has taken, held till t ends.
func freeDrive(t *testing.T) string {
	t.Helper()

	drivesMu.Lock()
	defer drivesMu.Unlock()
	for _, l := range "QRSTUVWXYZ" {
		d := string(l) + ":"
		if _, err := os.Stat(d + `\`); err != nil && !drives[d] {
			drives[d] = true
			t.Cleanup(func() { drivesMu.Lock(); delete(drives, d); drivesMu.Unlock() })
			return d
		}
	}
	t.Fatal("no free drive letter in Q to Z")
	return ""
}

var (
	floeOnce sync.Once
	floeExe  []byte
	floeErr  error
)

// builtFloe is floe for this machine, built as a release is, as version 0.1.0.
func builtFloe(t *testing.T) []byte {
	t.Helper()

	floeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "floe-install-test-build-")
		if err != nil {
			floeErr = err
			return
		}
		defer os.RemoveAll(dir)
		out := filepath.Join(dir, "floe.exe")
		cmd := exec.CommandContext(context.Background(), "go", "build", "-trimpath", "-ldflags", "-s -w -X floe/internal/version.Version=0.1.0", "-o", out, "./cmd/floe")
		cmd.Dir = "../.."
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if b, err := cmd.CombinedOutput(); err != nil {
			floeErr = fmt.Errorf("go build: %w\n%s", err, b)
			return
		}
		floeExe, floeErr = os.ReadFile(out)
	})
	if floeErr != nil {
		t.Fatal(floeErr)
	}
	return floeExe
}

func assetName(arch string) string { return "floe-windows-" + arch + ".exe" }

// goodRelease has this machine's floe as its asset and a text for the other machine's.
func goodRelease(t *testing.T) *ghRelease {
	t.Helper()

	r := &ghRelease{api: `{"tag_name": "v0.1.0"}`, files: map[string][]byte{}}
	for _, arch := range []string{"amd64", "arm64"} {
		if arch == runtime.GOARCH {
			r.files[assetName(arch)] = builtFloe(t)
		} else {
			r.files[assetName(arch)] = []byte("binary " + assetName(arch) + "\n")
		}
	}
	return r
}

// answer writes s to w. A client that went away is what some cases make: what is lost is not checked.
func answer(w io.Writer, s string) {
	_, _ = io.WriteString(w, s) //nolint:errcheck // see above
}

func sumOf(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// sumsOf is SHA256SUMS as the release writes it, with lines of other names around each that a loose match would take.
func (r *ghRelease) sumsOf() string {
	zero := strings.Repeat("0", 64)
	var b strings.Builder
	for _, name := range []string{"floe-linux-amd64", assetName("amd64"), assetName("arm64")} {
		sum := zero
		if body, ok := r.files[name]; ok {
			sum = sumOf(body)
		}
		b.WriteString(zero + "  " + name + ".sig\n" + zero + "  x" + name + "\n" + sum + "  " + name + "\n")
	}
	return b.String()
}

// An http server's answers, which install.ps1 must never ask for: an older release, and on /hop/ a way back to https.
const oldFloe = "floe 0.0.9 from http\n"

// site serves r as GitHub does, over TLS and through an HTTP proxy that tunnels every CONNECT to it. A plain request
// to the proxy is counted and answered as an http server would (oldFloe). It returns the proxy's URL.
// Each tunnel and request is logged with its time, for a test that fails: a child killed at its limit is read by
// them. Nothing is logged once a connection is hijacked: the servers' Close waits for the handlers before that only.
func site(t *testing.T, r *ghRelease) string {
	t.Helper()

	began := time.Now()
	seen := func(what string) { t.Logf("+%.1fs %s", time.Since(began).Seconds(), what) }
	plain := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen("in plain http: " + req.Method + " " + req.URL.Path)
		r.hits.Add(1)
		r.plain.Add(1)
		name := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		switch {
		case strings.Contains(req.URL.Path, "/hop/"):
			http.Redirect(w, req, githubURL+"real/"+name, http.StatusFound) //nolint:gosec // G710: the test's own server
		case strings.HasSuffix(req.URL.Path, "/releases/latest"):
			answer(w, `{"tag_name": "v0.0.9"}`)
		case name == "SHA256SUMS":
			answer(w, sumOf([]byte(oldFloe))+"  "+assetName("amd64")+"\n"+sumOf([]byte(oldFloe))+"  "+assetName("arm64")+"\n")
		default:
			answer(w, oldFloe)
		}
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen(req.Method + " " + req.URL.Path)
		r.hits.Add(1)
		host := req.Host
		name := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		if f, ok := r.serve[name]; ok {
			f(w, req, host)
			return
		}
		if strings.HasSuffix(req.URL.Path, "/releases/latest") {
			if r.apiStatus != 0 {
				w.WriteHeader(r.apiStatus)
			}
			answer(w, r.api)
			return
		}
		if name == "SHA256SUMS" {
			sums := r.sums
			if sums == "" {
				sums = r.sumsOf()
			}
			answer(w, sums)
			return
		}
		body, ok := r.files[name]
		if !ok || !strings.HasPrefix(req.URL.Path, repoPath) {
			http.NotFound(w, req)
			return
		}
		answer(w, string(body))
	})
	tlsServer := httptest.NewUnstartedServer(handler)
	tlsServer.StartTLS()
	t.Cleanup(tlsServer.Close)

	ended := t.Context().Done()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodConnect {
			if req.URL.Path == silentPath { // the driver's, not install.ps1's: not counted
				answer(w, strconv.Itoa(int(r.tunnels.Load()-r.spoken.Load())))
				return
			}
			plain(w, req)
			return
		}
		seen("CONNECT " + req.Host)
		up, err := (&net.Dialer{}).DialContext(req.Context(), "tcp", tlsServer.Listener.Addr().String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			_ = up.Close() //nolint:errcheck // nothing was sent on it
			http.Error(w, "no hijacking", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			_ = up.Close()
			return
		}
		r.loseMu.Lock()
		lost := len(r.lose) > 0 && r.lose[0] == req.Host
		if lost {
			r.lose = r.lose[1:]
		}
		r.loseMu.Unlock()
		if r.mute == nil {
			r.tunnels.Add(1) // before the 200: a client that has it sees the tunnel counted
		}
		answer(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		if lost && r.mute == nil {
			_ = up.Close()   //nolint:errcheck // nothing was sent on it
			<-ended          // the client waits for a TLS that does not come, until it gives up
			_ = conn.Close() //nolint:errcheck // nothing more is sent on it
			return
		}
		if r.mute != nil {
			_ = up.Close()   //nolint:errcheck // nothing was sent on it
			<-r.mute         // the client waits for a TLS that does not come
			_ = conn.Close() //nolint:errcheck // the test is over
			return
		}
		// What the client sent before the answer was read with its request: it is in rw, not on conn, and goes up first.
		sent := io.MultiReader(io.LimitReader(rw.Reader, int64(rw.Reader.Buffered())), conn)
		// A tunnel: either side may end it, so neither copy's error is a failure of the test.
		// A tunnel the client sent nothing on is what the stop of Windows PowerShell 5.1 looks like (driver.ps1).
		heard := &firstRead{r: sent, seen: func() { r.spoken.Add(1) }}
		go func() { _, _ = io.Copy(up, heard); _ = up.Close() }() //nolint:errcheck // see above
		_, _ = io.Copy(conn, up)                                  //nolint:errcheck // see above
		_ = conn.Close()                                          //nolint:errcheck // see above
	}))
	t.Cleanup(proxy.Close)
	return proxy.URL
}

// The answers a test puts in release.serve.
func redirect(to string) func(w http.ResponseWriter, r *http.Request, host string) {
	return func(w http.ResponseWriter, r *http.Request, _ string) { http.Redirect(w, r, to, http.StatusFound) }
}

func status(code int, body string) func(w http.ResponseWriter, r *http.Request, host string) {
	return func(w http.ResponseWriter, _ *http.Request, _ string) { w.WriteHeader(code); answer(w, body) }
}

// psLimit is how long one run of install.ps1 may take: Windows PowerShell stopped once for 13 minutes before it began
// TLS through the proxy, and the package's 15 minutes ran out with nothing told of where.
// LIMIT: go test without -v holds what a subtest told until TestInstallPS1 ends: when another run stops after one
// was killed and the package's time runs out, none of it is shown. Write it to os.Stderr too when killing, if two
// stop in one run.
const psLimit = 5 * time.Minute

// psWait is the seconds install.ps1 waits for an answer in a run whose case does not set them: long past what an
// answer takes (a whole run took 21.5 seconds at most, 12 side by side), short of psLimit, for a request Windows
// PowerShell lost to time out and be sent again (driver.ps1).
// LIMIT: the time of a run on CI was not measured. A run that times out with the client heard from is not sent
// again and fails: then raise it.
const psWait = "60"

// timeoutHint is what install.ps1 adds when PowerShell gave up on a silence: the cases look for it, not for the rest
// of the words. Whether a request is sent again does not read it (driver.ps1).
const timeoutHint = "FLOE_INSTALL_TIMEOUT sets how many seconds"

// psCase is one run of install.ps1: what the driver does (testdata/driver.ps1) and the environment.
type psCase struct {
	Proxy       string `json:"proxy"`
	Report      string `json:"report"`
	Script      string `json:"script"`
	Home        string `json:"home"`
	Form        string `json:"form"` // iex or file
	Run         string `json:"run"`  // a command run instead of install.ps1
	Cd          string `json:"cd"`
	Strict      bool   `json:"strict"`
	PathKind    string `json:"pathKind"` // the user's Path before: none, String, ExpandString, MultiString
	PathName    string `json:"pathName"`
	Path        string `json:"path"`
	Notice      bool   `json:"notice"`
	FailMove    bool   `json:"failMove"`
	FailCleanup bool   `json:"failCleanup"`
	FailCopy    bool   `json:"failCopy"`
	Tmp         string `json:"tmp"`       // TMP and TEMP while install.ps1 runs; "" is base\tmp, for the driver too
	RetryStop   bool   `json:"retryStop"` // runPS's: the driver sends a request lost as Windows PowerShell does once more

	madeStop bool // the case loses a request itself: a retry of it is not told in CI's summary

	env   map[string]string // FLOE_* and the machine's variables; "" removes one
	limit time.Duration     // how long the run may take; 0 is psLimit
}

// psResult is what one run left.
type psResult struct {
	code           int
	out, err, both string
	tmp            string // TMP and TEMP of the run
	report         struct {
		Eap, Progress, PathKind, Path, PathName string
		Strict, Notice                          bool
		Retried                                 int
		SilentFailed                            string // why the driver could not read the proxy's count
	}
}

func (r *psResult) String() string {
	return fmt.Sprintf("exit %d, report %+v\n%s", r.code, r.report, r.both)
}

// runPS runs install.ps1 in shell (powershell or pwsh) as c says, from a working directory of its own (base\w).
func runPS(t *testing.T, shell, base string, c psCase) *psResult {
	t.Helper()

	driver, err := filepath.Abs(filepath.Join("testdata", "driver.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	// The process has base\tmp; the driver gives install.ps1 c.Tmp, if any.
	res := &psResult{tmp: cmp.Or(c.Tmp, filepath.Join(base, "tmp"))}
	for _, dir := range []string{res.tmp, filepath.Join(base, "tmp"), filepath.Join(base, "w")} {
		if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
			t.Fatal(mkErr)
		}
	}
	if c.Script == "" {
		c.Script, err = filepath.Abs(filepath.Join("..", "..", "install.ps1"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if c.Form == "" {
		c.Form = "iex"
	}
	if c.PathKind == "" {
		c.PathKind = "none"
	}
	if c.PathName == "" {
		c.PathName = "Path"
	}
	c.Home = base
	c.Report = filepath.Join(base, "report.json")
	// A case that sets the seconds to wait looks at time-outs itself: none of its requests is sent again.
	_, own := c.env["FLOE_INSTALL_TIMEOUT"]
	c.RetryStop = !own
	js, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToUpper(k), "FLOE_") {
			continue
		}
		env[strings.ToUpper(k)] = k + "=" + v
	}
	set := func(k, v string) {
		if v == "" {
			delete(env, strings.ToUpper(k))
			return
		}
		env[strings.ToUpper(k)] = k + "=" + v
	}
	set("TMP", filepath.Join(base, "tmp"))
	set("TEMP", filepath.Join(base, "tmp"))
	set("USERPROFILE", filepath.Join(base, "profile"))
	set("PROCESSOR_ARCHITECTURE", strings.ToUpper(runtime.GOARCH))
	set("PROCESSOR_ARCHITEW6432", "")
	set("FLOE_TEST_CASE", string(js))
	set("FLOE_INSTALL_TIMEOUT", psWait)
	for k, v := range c.env {
		set(k, v)
	}

	limit := c.limit
	if limit == 0 {
		limit = psLimit
	}
	var out, errb, both, waiting bytes.Buffer
	err = runLimited(t.Context(), limit, func(ctx context.Context) *exec.Cmd {
		//nolint:gosec // G204: powershell or pwsh, and the test's driver
		cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-Command", ". '"+strings.ReplaceAll(driver, "'", "''")+"'")
		cmd.Dir = filepath.Join(base, "w")
		for _, kv := range env {
			cmd.Env = append(cmd.Env, kv)
		}
		cmd.Stdout = io.MultiWriter(&out, &both)
		cmd.Stderr = io.MultiWriter(&errb, &both)
		return cmd
	}, func() {
		// Before the child is killed: its end closes the tunnel, and what waited on it is gone from the list.
		_ = pprof.Lookup("goroutine").WriteTo(&waiting, 2) //nolint:errcheck // a buffer takes it all
	})
	if errors.Is(err, errOverLimit) {
		t.Fatalf("%s: %v\nits output so far:\n%s\nevery goroutine, before it was killed:\n%s", shell, err, both.String(), waiting.String())
	}
	// Windows PowerShell folds a long error at the width of a console; the folds are taken out.
	res.out, res.err, res.both = out.String(), strings.NewReplacer("\r\n", "", "\n", "").Replace(errb.String()), both.String()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		res.code = exit.ExitCode()
	case err != nil:
		t.Fatalf("%s: %v\n%s", shell, err, res.both)
	}
	b, err := os.ReadFile(c.Report)
	if err != nil {
		t.Fatalf("the driver wrote no report (%v):\n%s", err, res)
	}
	if err := json.Unmarshal(b, &res.report); err != nil {
		t.Fatalf("the driver's report %s: %v", b, err)
	}
	if res.report.SilentFailed != "" {
		t.Errorf("the driver could not read the proxy's count of silent tunnels, and sent nothing again: %s", res.report.SilentFailed)
	}
	if res.report.Retried > 0 {
		t.Logf("%s timed out on a tunnel it sent nothing on: the request was sent again", shell)
		if !c.madeStop {
			tellRetry(t)
		}
	}
	return res
}

// maxRetries is how many requests lost as Windows PowerShell 5.1 loses one may be sent again in a run of the
// package: past it, a stop comes every time (install.ps1, the test's proxy), not now and then.
// LIMIT: 3 in a run of some 100 Windows PowerShell runs, each lost about once in 4600 (seen on CI), come less than
// once in 500000 runs of the package. Raise it if Windows PowerShell comes to lose more.
const maxRetries = 2

var (
	retriesMu sync.Mutex
	retries   int
)

// tellRetry counts a request sent again for t, failing t past maxRetries, and adds a line for it to the summary of
// the CI step running it (GITHUB_STEP_SUMMARY): go test shows nothing of a package that passed. Elsewhere, with no
// such file, it adds none.
func tellRetry(t *testing.T) {
	t.Helper()
	retriesMu.Lock() // a line at a time: the runs go side by side
	defer retriesMu.Unlock()
	retries++
	if retries > maxRetries {
		t.Errorf("%d requests were sent again in this run, more than %d: Windows PowerShell loses one now and then, not so often", retries, maxRetries)
	}
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: the file CI names
	if err == nil {
		_, err = fmt.Fprintf(f, "- `%s`: Windows PowerShell sent nothing on a tunnel and timed out; the request was sent again\n", t.Name())
		err = errors.Join(err, f.Close())
	}
	if err != nil {
		t.Errorf("the retry could not be told in the step's summary: %v", err)
	}
}

// shells are the PowerShells the test runs: Windows PowerShell is part of Windows; PowerShell 7 must be there on CI.
func shells(t *testing.T) []string {
	t.Helper()

	got := []string{}
	for _, shell := range []string{"powershell", "pwsh"} {
		if _, err := exec.LookPath(shell); err != nil {
			if shell == "pwsh" && os.Getenv("GITHUB_ACTIONS") == "" {
				t.Logf("pwsh is not on PATH: PowerShell 7 is not tested here")
				continue
			}
			t.Fatalf("%s is not on PATH", shell)
		}
		got = append(got, shell)
	}
	return got
}

// installed is the file at dir\floe.exe, or nil.
func installed(t *testing.T, dir string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, "floe.exe"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// leftovers are the names in dir that install.ps1 makes for a moment: floe.exe.new-* and floe.exe.old-*.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "floe.exe.new-") || strings.HasPrefix(e.Name(), "floe.exe.old-") {
			got = append(got, e.Name())
		}
	}
	return got
}

// TestInstallPS1 runs install.ps1 in Windows PowerShell 5.1 and PowerShell 7, as `irm | iex` runs it and as a file,
// with the real Invoke-WebRequest against a GitHub of the test.
func TestInstallPS1(t *testing.T) {
	t.Parallel()

	for _, shell := range shells(t) {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			testInstallPS1(t, shell)
		})
	}
}

func testInstallPS1(t *testing.T, shell string) {
	t.Helper()

	host := runtime.GOARCH
	other := map[string]string{"amd64": "arm64", "arm64": "amd64"}[host]
	zero := strings.Repeat("0", 64)

	// A failure: exit code 1, said says each of says, nothing at dir\floe.exe, and nothing left in the temp folder.
	fails := func(t *testing.T, res *psResult, dir string, says ...string) {
		t.Helper()
		if res.code != 1 {
			t.Errorf("exit code %d, want 1:\n%s", res.code, res)
		}
		for _, s := range says {
			if !strings.Contains(res.err, s) {
				t.Errorf("stderr does not say %q:\n%s", s, res)
			}
		}
		if b := installed(t, dir); b != nil {
			t.Errorf("floe.exe is installed although install.ps1 failed:\n%s", res)
		}
		if left, err := os.ReadDir(res.tmp); err != nil || len(left) != 0 {
			t.Errorf("the temp folder holds %d entries after a failure (%v)", len(left), err)
		}
	}
	// A success: exit code 0 and dir\floe.exe is the asset of arch.
	works := func(t *testing.T, r *ghRelease, res *psResult, dir, arch string) {
		t.Helper()
		if res.code != 0 {
			t.Fatalf("exit code %d, want 0:\n%s", res.code, res)
		}
		if got := installed(t, dir); !bytes.Equal(got, r.files[assetName(arch)]) {
			t.Errorf("dir\\floe.exe is not %s (%d bytes):\n%s", assetName(arch), len(got), res)
		}
		if left, err := os.ReadDir(res.tmp); err != nil || len(left) != 0 {
			t.Errorf("the temp folder holds %d entries (%v)", len(left), err)
		}
	}
	install := func(t *testing.T, r *ghRelease, c psCase) (*psResult, string) {
		t.Helper()
		base := t.TempDir()
		c.Proxy = site(t, r)
		dir := filepath.Join(base, "bin")
		if c.env == nil {
			c.env = map[string]string{}
		}
		if _, ok := c.env["FLOE_INSTALL_DIR"]; !ok {
			c.env["FLOE_INSTALL_DIR"] = dir
		}
		if d := c.env["FLOE_INSTALL_DIR"]; d != "" && filepath.IsAbs(d) {
			dir = d
		}
		return runPS(t, shell, base, c), dir
	}

	t.Run("latest, a real floe", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		res, dir := install(t, r, psCase{})
		works(t, r, res, dir, host)
		out, err := exec.CommandContext(t.Context(), filepath.Join(dir, "floe.exe"), "version").CombinedOutput() //nolint:gosec // G204: the floe the test installed
		if err != nil || strings.TrimSpace(string(out)) != "floe 0.1.0" {
			t.Errorf("floe.exe version: %q %v", out, err)
		}
		if !strings.Contains(res.out, "installed floe 0.1.0 to "+filepath.Join(dir, "floe.exe")) {
			t.Errorf("install.ps1 does not say where:\n%s", res)
		}
		if res.report.Eap != "Continue" || res.report.Progress != "Continue" || res.report.Strict {
			t.Errorf("the caller's settings changed: %+v", res.report)
		}
	})
	t.Run("as a file", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		res, dir := install(t, r, psCase{Form: "file"})
		works(t, r, res, dir, host)
	})
	for _, v := range []struct{ name, version string }{{"pinned", "0.1.0"}, {"pinned with v", "v0.1.0"}, {"latest by name", "latest"}, {"empty", ""}} {
		t.Run("version "+v.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			r.api = `{"tag_name": "v9.9.9"}` // a pinned one never asks
			if v.version == "latest" || v.version == "" {
				r.api = `{"tag_name": "v0.1.0"}`
			}
			res, dir := install(t, r, psCase{env: map[string]string{"FLOE_VERSION": v.version}})
			works(t, r, res, dir, host)
			if !strings.Contains(res.out, "downloading "+assetName(host)+"@0.1.0") {
				t.Errorf("not 0.1.0:\n%s", res)
			}
		})
	}
	for _, a := range []struct{ name, arch, w6432, want string }{
		{"arm64", "ARM64", "", "arm64"},
		{"32-bit on amd64", "x86", "AMD64", "amd64"},
		{"32-bit on arm64", "x86", "ARM64", "arm64"},
		{"the other one", strings.ToUpper(other), "", other},
	} {
		t.Run("arch "+a.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"PROCESSOR_ARCHITECTURE": a.arch, "PROCESSOR_ARCHITEW6432": a.w6432}})
			works(t, r, res, dir, a.want)
		})
	}
	for _, a := range []struct{ name, arch string }{{"x86 alone", "x86"}, {"unknown", "IA64"}, {"lower case", "amd64"}} {
		t.Run("arch "+a.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"PROCESSOR_ARCHITECTURE": a.arch}})
			fails(t, res, dir, "unsupported architecture", "amd64 and arm64")
			if r.hits.Load() != 0 {
				t.Errorf("%d requests", r.hits.Load())
			}
		})
	}

	t.Run("the default folder", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		dir := filepath.Join(base, "profile", ".local", "bin")
		// The folder is shared with other programs: what is in it stays, names like floe's too.
		others := map[string]string{"claude.exe": "c", "floe.exe.config": "f", "floe.exe.old": "o"}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for name, body := range others {
			write(t, filepath.Join(dir, name), body)
		}
		res := runPS(t, shell, base, psCase{Proxy: site(t, r), env: map[string]string{"FLOE_INSTALL_DIR": "", "FLOE_INSTALL_TIMEOUT": ""}})
		works(t, r, res, dir, host)
		for name, body := range others {
			if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != body {
				t.Errorf("%s is %q (%v), want %q as it was", name, b, err, body)
			}
		}
		// Not on PATH: the line shown names no folder, as the default is where it goes.
		shown := after(strings.ReplaceAll(res.out, "\r\n", "\n"), "To add it, run:\n")
		if !strings.Contains(shown, "$env:FLOE_ADD_TO_PATH = '1'; irm ") || strings.Contains(shown, "FLOE_INSTALL_DIR") ||
			strings.Contains(shown, "FLOE_INSTALL_TIMEOUT") {
			t.Errorf("the line shown is %q, want FLOE_ADD_TO_PATH and irm, no FLOE_INSTALL_DIR nor FLOE_INSTALL_TIMEOUT:\n%s", shown, res)
		}
	})
	t.Run("the default folder off a local drive", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		profile := `\\localhost\c$\floe-install-test-profile`
		at := `C:\floe-install-test-profile`
		// Also what an install that only made the folder would leave.
		t.Cleanup(func() { _ = os.RemoveAll(at) }) //nolint:errcheck // a folder of the test's own
		res := runPS(t, shell, base, psCase{Proxy: site(t, r), env: map[string]string{"FLOE_INSTALL_DIR": "", "USERPROFILE": profile}})
		fails(t, res, filepath.Join(base, "profile", ".local", "bin"), "(got "+profile+`\.local\bin)`, "FLOE_INSTALL_DIR")
		if r.hits.Load() != 0 {
			t.Errorf("%d requests", r.hits.Load())
		}
		if installed(t, filepath.Join(at, ".local", "bin")) != nil {
			t.Errorf("floe.exe was put under %s", at)
		}
	})
	t.Run("no USERPROFILE", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		res := runPS(t, shell, base, psCase{Proxy: site(t, r), env: map[string]string{"FLOE_INSTALL_DIR": "", "USERPROFILE": ""}})
		fails(t, res, filepath.Join(base, "profile", ".local", "bin"), "USERPROFILE", "FLOE_INSTALL_DIR")
	})
	t.Run("relative, from where you are", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		there := filepath.Join(base, "there")
		if err := os.Mkdir(there, 0o700); err != nil {
			t.Fatal(err)
		}
		res := runPS(t, shell, base, psCase{Proxy: site(t, r), Cd: there, env: map[string]string{"FLOE_INSTALL_DIR": "bin"}})
		works(t, r, res, filepath.Join(there, "bin"), host)
		if _, err := os.Stat(filepath.Join(base, "w", "bin")); err == nil {
			t.Error("a bin was made where PowerShell started, not where it was told to go")
		}
	})
	t.Run("brackets in the folder", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		dir := filepath.Join(t.TempDir(), "a[1]")
		res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir}})
		works(t, r, res, dir, host)
	})
	for _, name := range []string{"tmp[1]", "tmp[", "tmp]"} {
		t.Run("TMP "+name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{Tmp: filepath.Join(t.TempDir(), name)})
			works(t, r, res, dir, host)
		})
	}
	// at is where a refused folder would be on this machine: nothing may be put there.
	for _, d := range []struct{ name, dir, says, at string }{
		{"UNC", `\\localhost\c$\floe-install-test-unc`, "local drive", `C:\floe-install-test-unc`},
		{"UNC with slashes", `//localhost/c$/floe-install-test-slashes`, "local drive", `C:\floe-install-test-slashes`},
		{"a device path", `\\?\C:\floe-install-test-device`, "local drive", `C:\floe-install-test-device`},
		{"no such drive", `Q:foo`, "Q", ""},
		{"the registry", `HKCU:\floe-install-test`, "local drive", ""},
	} {
		t.Run("folder "+d.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			if d.name == "no such drive" {
				drive := freeDrive(t)
				d.dir = drive + "foo"
				d.says = "'" + strings.TrimSuffix(drive, ":") + "'"
			}
			res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": d.dir}})
			fails(t, res, t.TempDir(), d.says)
			if r.hits.Load() != 0 {
				t.Errorf("%d requests", r.hits.Load())
			}
			if d.at != "" && installed(t, d.at) != nil {
				t.Errorf("floe.exe was put at %s", d.at)
				_ = os.RemoveAll(d.at) //nolint:errcheck // what the failure left
			}
		})
	}
	t.Run("folder under a file", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		file := filepath.Join(t.TempDir(), "file")
		write(t, file, "")
		res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": filepath.Join(file, "bin")}})
		fails(t, res, filepath.Join(file, "bin"), "FLOE_INSTALL_DIR")
	})

	// SHA256SUMS, as the release writes it and otherwise. H is the asset's sum, N its name.
	for _, s := range []struct {
		name, sums string
		works      bool
		says       string
	}{
		{"its line first", "H  N\n" + zero + "  xN\n", true, ""},
		{"no line break at the end", zero + "  N.sig\nH  N", true, ""},
		{"upper case", "UPPER  N\n", false, "checksum mismatch"},
		{"CRLF", "H  N\r\n", false, "not listed"},
		{"twice, the same", "H  N\nH  N\n", false, "checksum mismatch"},
		{"twice, one wrong", zero + "  N\nH  N\n", false, "checksum mismatch"},
		{"binary mark", "H *N\n", false, "not listed"},
		{"one space", "H N\n", false, "not listed"},
		{"a tab", "H\tN\n", false, "not listed"},
		{"a space at the end", "H  N \n", false, "not listed"},
		{"the name in upper case", "H  UPPERNAME\n", false, "not listed"},
		{"a BOM", "\ufeffH  N\n", false, "checksum mismatch"},
		{"a space in front", "  H  N\n", false, "checksum mismatch"},
		{"three spaces", "H   N\n", false, "checksum mismatch"},
		{"another field", "H junk  N\n", false, "checksum mismatch"},
		{"a zero-width space", "H\u200b  N\n", false, "checksum mismatch"},
		{"no line", zero + "  xN\n", false, "not listed"},
		{"empty", "", false, "not listed"},
		{"a page", "<!DOCTYPE html><html><body>Not here</body></html>\n", false, "not listed"},
		{"63 letters", "SHORT  N\n", false, "checksum mismatch"},
		{"65 letters", "H0  N\n", false, "checksum mismatch"},
		{"wrong", zero + "  N\n", false, "checksum mismatch"},
	} {
		t.Run("SHA256SUMS "+s.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			sum := sumOf(r.files[assetName(host)])
			r.sums = strings.NewReplacer("UPPERNAME", strings.ToUpper(assetName(host)), "UPPER", strings.ToUpper(sum),
				"SHORT", sum[:63], "H", sum, "N", assetName(host)).Replace(s.sums)
			if r.sums == "" {
				r.sums = "\n"
			}
			res, dir := install(t, r, psCase{})
			if s.works {
				works(t, r, res, dir, host)
				return
			}
			fails(t, res, dir, s.says)
		})
	}

	broken := func(t *testing.T, name string, f func(w http.ResponseWriter, r *http.Request, host string)) *ghRelease {
		t.Helper()
		r := goodRelease(t)
		r.serve = map[string]func(w http.ResponseWriter, r *http.Request, host string){name: f}
		return r
	}
	asset := assetName(host)
	for _, b := range []struct {
		name, file string
		serve      func(w http.ResponseWriter, r *http.Request, host string)
		says       string
	}{
		{"cut short", asset, func(w http.ResponseWriter, _ *http.Request, _ string) {
			w.Header().Set("Content-Length", "100000")
			answer(w, "binary")
		}, ""},
		{"empty", asset, status(http.StatusOK, ""), "checksum mismatch"},
		{"a page", asset, status(http.StatusOK, "<html><body>rate limited</body></html>"), "checksum mismatch"},
		{"404", asset, status(http.StatusNotFound, "<html>Not Found</html>"), "(status 404)"},
		{"the asset to http", asset, redirect("http://github.com" + repoPath + "v0.1.0/" + asset), "download failed: " + githubURL + "v0.1.0/" + asset + " (redirected to http)"},
		{"SHA256SUMS to http", "SHA256SUMS", redirect("http://github.com" + repoPath + "v0.1.0/SHA256SUMS"), "download failed: " + githubURL + "v0.1.0/SHA256SUMS (redirected to http)"},
		{"http in the middle", asset, redirect("http://github.com" + repoPath + "hop/" + asset), "download failed: " + githubURL + "v0.1.0/" + asset + " (redirected to http)"},
		{"to nowhere", asset, func(w http.ResponseWriter, _ *http.Request, _ string) { w.WriteHeader(http.StatusFound) }, "(redirected to nowhere)"},
		{"a loop", asset, redirect(githubURL + "v0.1.0/" + asset), "(too many redirects)"},
	} {
		t.Run("download "+b.name, func(t *testing.T) {
			t.Parallel()
			r := broken(t, b.file, b.serve)
			res, dir := install(t, r, psCase{})
			fails(t, res, dir, b.says)
			if b.name == "404" && !strings.Contains(res.err, "check that floe 0.1.0 has a Windows build: https://github.com/sukekyo26/floe/releases") {
				t.Errorf("a missing floe.exe does not say to check the version has a Windows build:\n%s", res)
			}
			if r.plain.Load() != 0 {
				t.Errorf("%d requests in plain http", r.plain.Load())
			}
		})
	}
	// What install.ps1 adds when PowerShell gave up on a silence, and what it says of a FLOE_INSTALL_TIMEOUT it refuses.
	const waited = "check your network and proxy and run it again; " + timeoutHint + " to wait for an answer (now 3)"
	const refused = "FLOE_INSTALL_TIMEOUT must be"
	// A server or a proxy that goes silent is given up on after FLOE_INSTALL_TIMEOUT seconds, and told as that. A
	// run that does not give up is killed at a minute, not at psLimit.
	for _, s := range []struct {
		name string
		make func(t *testing.T, hold <-chan struct{}) *ghRelease
		skip bool
	}{
		{"no answer to a request", func(t *testing.T, hold <-chan struct{}) *ghRelease {
			t.Helper()
			return broken(t, asset, func(http.ResponseWriter, *http.Request, string) { <-hold })
		}, false},
		{"no TLS behind the proxy", func(t *testing.T, hold <-chan struct{}) *ghRelease {
			t.Helper()
			r := goodRelease(t)
			r.mute = hold
			return r
		}, false},
		// LIMIT: Windows PowerShell has nothing to pass for a body that stops: .NET gives up on it after 5 minutes,
		// whatever FLOE_INSTALL_TIMEOUT is (seen once, run 44263536485). Not run: it would take those.
		{"a body that stops", func(t *testing.T, hold <-chan struct{}) *ghRelease {
			t.Helper()
			return broken(t, asset, func(w http.ResponseWriter, _ *http.Request, _ string) {
				w.Header().Set("Content-Length", "100000")
				answer(w, strings.Repeat("x", 1000))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				<-hold
			})
		}, shell == "powershell"},
	} {
		t.Run("silence: "+s.name, func(t *testing.T) {
			t.Parallel()
			if s.skip {
				t.Skip("Windows PowerShell waits 5 minutes for a body that stops")
			}
			hold := make(chan struct{})
			defer close(hold) // before the servers close, which wait for their handlers
			r := s.make(t, hold)
			res, dir := install(t, r, psCase{limit: time.Minute, env: map[string]string{"FLOE_INSTALL_TIMEOUT": "3"}})
			fails(t, res, dir, "download failed: ", waited)
			if res.report.Retried != 0 {
				t.Errorf("a request was sent again %d times:\n%s", res.report.Retried, res)
			}
		})
	}
	// Windows PowerShell 5.1 now and then sends nothing on a tunnel the proxy opened, and the request times out
	// (driver.ps1). Made by a proxy that loses the CONNECTs named, such a request is sent again: once in a run, in a
	// case that does not set the wait, on that time-out, on a tunnel heard nothing on. PowerShell 7's time-out is
	// not one of Windows PowerShell's: its cases are the ones that fail when any other time-out is sent again.
	const api, gh = "api.github.com:443", "github.com:443"
	for _, s := range []struct {
		name    string
		lose    []string
		deaf    bool   // the server does not answer for floe.exe
		wait    string // FLOE_INSTALL_TIMEOUT; "" is runPS's
		retried int    // in Windows PowerShell
		works   bool   // in Windows PowerShell; a failure is a time-out of floe.exe's request
		pwsh    bool   // run in PowerShell 7 too, where nothing is sent again
	}{
		// In PowerShell 7, this one fails when a time-out that is not Windows PowerShell's is sent again.
		{"once, then it works", []string{gh}, false, "", 1, true, true},
		{"every time", []string{gh, gh}, false, "", 1, false, false},
		{"two requests", []string{api, gh}, false, "", 1, false, false},
		{"no tunnel left silent", nil, true, "", 0, false, false},
		{"a wait of its own", []string{gh}, false, "3", 0, false, false},
	} {
		if shell != "powershell" && !s.pwsh {
			continue
		}
		t.Run("stop: "+s.name, func(t *testing.T) {
			t.Parallel()
			hold := make(chan struct{})
			defer close(hold) // before the servers close, which wait for their handlers
			r := goodRelease(t)
			if s.deaf {
				r = broken(t, asset, func(http.ResponseWriter, *http.Request, string) { <-hold })
			}
			r.lose = s.lose
			c := psCase{madeStop: true}
			if s.wait != "" {
				c.env = map[string]string{"FLOE_INSTALL_TIMEOUT": s.wait}
			}
			res, dir := install(t, r, c)
			retried, installs := s.retried, s.works
			if shell != "powershell" {
				retried, installs = 0, false
			}
			if res.report.Retried != retried {
				t.Errorf("a request was sent again %d times, want %d:\n%s", res.report.Retried, retried, res)
			}
			if installs {
				works(t, r, res, dir, host)
			} else {
				// floe.exe's: in "two requests" the API's, sent again, came back.
				fails(t, res, dir, "download failed: "+githubURL+"v0.1.0/"+asset+" (", timeoutHint)
			}
		})
	}
	t.Run("slow, never silent", func(t *testing.T) {
		t.Parallel()
		// 14 seconds of a body under a limit of 10: the limit is on a silence, not on the whole download. A second
		// between two writes is a tenth of it, so a runner that lags does not make one.
		r := goodRelease(t)
		body := r.files[asset]
		r.serve = map[string]func(w http.ResponseWriter, r *http.Request, host string){asset: func(w http.ResponseWriter, _ *http.Request, _ string) {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			const parts = 14
			for i := range parts {
				_, _ = w.Write(body[len(body)*i/parts : len(body)*(i+1)/parts]) //nolint:errcheck // a client that went away fails the case by itself
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(time.Second)
			}
		}}
		res, dir := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_TIMEOUT": "10"}})
		works(t, r, res, dir, host)
	})
	// Read as letters, not as a number: what sh and PowerShell would read apart is refused by both alike, before
	// anything is asked for.
	for _, v := range []string{"abc", "0", "08", "-5", "+5", "1.5", "1e3", "0x10", " 5", "100000", "5\n", "\uff15"} {
		// %+q: the name is in the path of the run's temp folder, and Windows PowerShell's Add-Type (the driver's)
		// finds no path with a letter outside the system's code page in it.
		t.Run(fmt.Sprintf("refuses to wait %+q", v), func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_TIMEOUT": v}})
			// Windows PowerShell writes a letter outside the console's code page as another: only ASCII is looked for.
			got := "(got " + strings.TrimRight(v, "\n")
			if v == "\uff15" {
				got = "(got "
			}
			fails(t, res, dir, refused, got)
			if n := r.hits.Load(); n != 0 {
				t.Errorf("%d requests before FLOE_INSTALL_TIMEOUT was refused", n)
			}
		})
	}
	for _, v := range []string{"600", "99999", ""} {
		t.Run(fmt.Sprintf("waits %q", v), func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_TIMEOUT": v}})
			works(t, r, res, dir, host)
		})
	}
	t.Run("a second to wait is taken", func(t *testing.T) {
		t.Parallel()
		// Whether every request answers in a second on a busy runner is not the test's to say: only that 1 is read.
		res, _ := install(t, goodRelease(t), psCase{env: map[string]string{"FLOE_INSTALL_TIMEOUT": "1"}})
		if strings.Contains(res.err, refused) {
			t.Errorf("FLOE_INSTALL_TIMEOUT=1 is refused:\n%s", res)
		}
	})
	for _, b := range []struct {
		name  string
		serve func(w http.ResponseWriter, r *http.Request, host string)
	}{
		{"to another host", redirect("https://release-assets.githubusercontent.com" + repoPath + "real/" + asset)},
		{"twice", redirect("https://objects.githubusercontent.com" + repoPath + "hop2/" + asset)},
		{"relative", redirect(repoPath + "real/" + asset)},
		{"307", func(w http.ResponseWriter, r *http.Request, _ string) {
			http.Redirect(w, r, "https://release-assets.githubusercontent.com"+repoPath+"real/"+asset, http.StatusTemporaryRedirect)
		}},
		// A signed URL is taken as it is written: PowerShell 7's Uri.ToString() would make it %7C.
		{"an escape as written", redirect("https://release-assets.githubusercontent.com" + repoPath + "real/" + asset + "?sig=a%7cb")},
	} {
		t.Run("download redirected "+b.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			good := r.files[asset]
			// The first request for the asset is redirected; the request it is sent on to is answered.
			var n atomic.Int32
			r.serve = map[string]func(w http.ResponseWriter, req *http.Request, host string){asset: func(w http.ResponseWriter, req *http.Request, host string) {
				switch {
				case strings.Contains(req.URL.Path, "/hop2/"):
					redirect("https://release-assets.githubusercontent.com"+repoPath+"real/"+asset)(w, req, host)
				case n.Add(1) == 1:
					b.serve(w, req, host)
				case req.URL.RawQuery != "" && req.URL.RawQuery != "sig=a%7cb":
					http.Error(w, "the query is "+req.URL.RawQuery, http.StatusBadRequest)
				default:
					answer(w, string(good))
				}
			}}
			res, dir := install(t, r, psCase{})
			works(t, r, res, dir, host)
		})
	}

	for _, a := range []struct {
		name, api string
		code      int
		says      []string
	}{
		{"a page", "<html><body>rate limited</body></html>", 0, []string{"could not parse tag_name"}},
		{"cut short", `{"tag_name": "v0.1.0"`, 0, []string{"could not parse tag_name"}},
		{"empty", "", 0, []string{"could not parse tag_name"}},
		{"no tag_name", `{"message": "hi"}`, 0, []string{"could not parse tag_name"}},
		{"a number", `{"tag_name": 5}`, 0, []string{"could not parse tag_name"}},
		{"a list", `{"tag_name": ["v1", "v2"]}`, 0, []string{"could not parse tag_name"}},
		{"403", `{"message": "API rate limit exceeded"}`, http.StatusForbidden, []string{"(status 403)", "FLOE_VERSION"}},
		{"a folding tag", `{"tag_name": "v../../x"}`, 0, []string{"unsupported version", "FLOE_VERSION"}},
	} {
		t.Run("API "+a.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			r.api, r.apiStatus = a.api, a.code
			res, dir := install(t, r, psCase{})
			fails(t, res, dir, a.says...)
		})
	}
	t.Run("API to http", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		r.apiStatus, r.api = http.StatusFound, ""
		r.serve = map[string]func(w http.ResponseWriter, req *http.Request, host string){"latest": redirect("http://api.github.com/repos/sukekyo26/floe/releases/latest")}
		res, dir := install(t, r, psCase{})
		fails(t, res, dir, "download failed: https://api.github.com/repos/sukekyo26/floe/releases/latest (redirected to http)")
		if r.plain.Load() != 0 {
			t.Errorf("%d requests in plain http", r.plain.Load())
		}
	})
	t.Run("API no answer", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		r.serve = map[string]func(w http.ResponseWriter, req *http.Request, host string){"latest": func(w http.ResponseWriter, _ *http.Request, _ string) {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("the test's server cannot hijack")
				return
			}
			if conn, _, err := hijacker.Hijack(); err == nil {
				_ = conn.Close() //nolint:errcheck // closing is the answer
			}
		}}
		res, dir := install(t, r, psCase{})
		fails(t, res, dir, "download failed: https://api.github.com/repos/sukekyo26/floe/releases/latest")
		if strings.Contains(res.err, "rate limit") {
			t.Errorf("a failure with no answer is said to be a missing release or the rate limit:\n%s", res)
		}
	})
	t.Run("strict mode, no tag_name", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		r.api = `{"message": "hi"}`
		res, dir := install(t, r, psCase{Strict: true})
		fails(t, res, dir, "could not parse tag_name")
		if !res.report.Strict {
			t.Error("the caller's strict mode is off after install.ps1")
		}
	})

	for _, v := range []string{"../../x", "a/b", `a\b`, "1..2", " 0.1.0", "0.1.0\n", "\uff10.1.0", "v", "0.1.0?x=1", "%2e%2e"} {
		// %+q: a name not in ASCII goes into the temporary folder's name, where 5.1's Add-Type in the driver cannot compile.
		t.Run(fmt.Sprintf("version %+q", v), func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"FLOE_VERSION": v}})
			fails(t, res, dir, "unsupported version", "FLOE_VERSION")
			if r.hits.Load() != 0 {
				t.Errorf("%d requests for a version refused", r.hits.Load())
			}
		})
	}
	for _, v := range []string{"0.2.0-rc.1", "1.0.0+build.5"} {
		t.Run("version "+v, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"FLOE_VERSION": v}})
			works(t, r, res, dir, host)
		})
	}
	for _, v := range []string{"0", "true", " 1", "yes"} {
		t.Run(fmt.Sprintf("FLOE_ADD_TO_PATH %q", v), func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			res, dir := install(t, r, psCase{env: map[string]string{"FLOE_ADD_TO_PATH": v}})
			fails(t, res, dir, "FLOE_ADD_TO_PATH must be 1")
			if r.hits.Load() != 0 {
				t.Errorf("%d requests", r.hits.Load())
			}
		})
	}

	testPlacePS1(t, install, works)
	testPathPS1(t, shell, install, works)
}

// testPlacePS1 is putting floe.exe in place: one that runs, one held open, and a rename that fails.
func testPlacePS1(t *testing.T, install func(*testing.T, *ghRelease, psCase) (*psResult, string), works func(*testing.T, *ghRelease, *psResult, string, string)) {
	t.Helper()

	host := runtime.GOARCH

	t.Run("held open", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		dir := filepath.Join(t.TempDir(), "bin")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, "floe.exe"), "old")
		// os.Open does not share delete, so floe.exe cannot be renamed while it is open.
		f, err := os.Open(filepath.Join(dir, "floe.exe"))
		if err != nil {
			t.Fatal(err)
		}
		res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir}})
		_ = f.Close()
		if res.code != 1 || !strings.Contains(res.err, "cannot replace") {
			t.Errorf("want a failure that says cannot replace:\n%s", res)
		}
		if got := string(installed(t, dir)); got != "old" {
			t.Errorf("floe.exe holds %q, want the old one", got)
		}
		if left := leftovers(t, dir); len(left) != 0 {
			t.Errorf("left %q", left)
		}
	})
	t.Run("the last rename fails", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		dir := filepath.Join(t.TempDir(), "bin")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, "floe.exe"), "old")
		res, _ := install(t, r, psCase{FailMove: true, env: map[string]string{"FLOE_INSTALL_DIR": dir}})
		if res.code != 1 || !strings.Contains(res.err, "cannot replace") {
			t.Errorf("want a failure that says cannot replace:\n%s", res)
		}
		if got := string(installed(t, dir)); got != "old" {
			t.Errorf("floe.exe holds %q, want the old one put back", got)
		}
		if left := leftovers(t, dir); len(left) != 0 {
			t.Errorf("left %q", left)
		}
	})
	t.Run("running", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		dir := filepath.Join(t.TempDir(), "bin")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "floe.exe"), b, 0o700); err != nil { //nolint:gosec // G306: a program to run
			t.Fatal(err)
		}
		running := exec.CommandContext(t.Context(), filepath.Join(dir, "floe.exe")) //nolint:gosec // G204: this test binary, copied
		running.Env = append(os.Environ(), "FLOE_INSTALL_TEST_SLEEP=1")
		if err := running.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = running.Process.Kill(), running.Wait() }) //nolint:errcheck // ended by now, or killed

		// The second install moves aside the floe.exe the first put, which does not run, and removes it.
		for i := range 2 {
			res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir}})
			works(t, r, res, dir, host)
			if left := leftovers(t, dir); len(left) != 1 {
				t.Errorf("install %d left %q, want the one that runs moved aside", i+1, left)
			}
		}
		if err := running.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = running.Wait() //nolint:errcheck // killed: it says so
		res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir}})
		works(t, r, res, dir, host)
		if left := leftovers(t, dir); len(left) != 0 {
			t.Errorf("left %q after the old one ended", left)
		}
	})
	t.Run("the copy cut short", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		dir := filepath.Join(t.TempDir(), "bin")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, "floe.exe"), "old")
		res, _ := install(t, r, psCase{FailCopy: true, env: map[string]string{"FLOE_INSTALL_DIR": dir}})
		if res.code != 1 || !strings.Contains(res.err, "cannot write") {
			t.Errorf("want a failure that says cannot write:\n%s", res)
		}
		if got := string(installed(t, dir)); got != "old" {
			t.Errorf("floe.exe holds %q, want the old one", got)
		}
		if left := leftovers(t, dir); len(left) != 0 {
			t.Errorf("left %q", left)
		}
	})
	t.Run("the temp folder held", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		res, dir := install(t, r, psCase{FailCleanup: true})
		if res.code != 0 || installed(t, dir) == nil {
			t.Errorf("a temp folder that cannot be removed fails a finished install:\n%s", res)
		}
		r = goodRelease(t)
		r.sums = strings.Repeat("0", 64) + "  " + assetName(host) + "\n"
		res, _ = install(t, r, psCase{FailCleanup: true})
		if res.code != 1 || !strings.Contains(res.err, "checksum mismatch") {
			t.Errorf("a temp folder that cannot be removed hides why it failed:\n%s", res)
		}
	})
}

// testPathPS1 is the user's PATH: what is said, what is written with FLOE_ADD_TO_PATH=1, and the line shown, run.
func testPathPS1(t *testing.T, shell string, install func(*testing.T, *ghRelease, psCase) (*psResult, string), works func(*testing.T, *ghRelease, *psResult, string, string)) {
	t.Helper()

	host := runtime.GOARCH

	// Written with FLOE_ADD_TO_PATH=1: the Path before, its kind and name, and after. D is the folder.
	for _, p := range []struct {
		name, kind, path, after, afterKind, pathName string
	}{
		{"no Path", "none", "", `D`, "ExpandString", ""},
		{"empty, a String", "String", "", `D`, "String", ""},
		{"empty, an ExpandString", "ExpandString", "", `D`, "ExpandString", ""},
		{"one, a String", "String", `C:\a`, `C:\a;D`, "String", ""},
		{"a ; at the end", "ExpandString", `C:\a;C:\b;`, `C:\a;C:\b;D`, "ExpandString", ""},
		{"a ;; inside", "ExpandString", `C:\a;;C:\b`, `C:\a;;C:\b;D`, "ExpandString", ""},
		{"entries with variables", "ExpandString", `%USERPROFILE%\bin;%JAVA_HOME%\bin;C:\b`, `%USERPROFILE%\bin;%JAVA_HOME%\bin;C:\b;D`, "ExpandString", ""},
		{"there", "ExpandString", `C:\a;D;C:\b`, `C:\a;D;C:\b`, "ExpandString", ""},
		{"there, another case and a \\", "ExpandString", `C:\a;DUPPER\;C:\b`, `C:\a;DUPPER\;C:\b`, "ExpandString", ""},
		{"there in quotes", "String", `"D";C:\a`, `"D";C:\a`, "String", ""},
		{"there as a variable", "ExpandString", `C:\a;%FLOE_TEST_HOME%\bin`, `C:\a;%FLOE_TEST_HOME%\bin`, "ExpandString", ""},
		{"a variable in a String is not opened", "String", `C:\a;%FLOE_TEST_HOME%\bin`, `C:\a;%FLOE_TEST_HOME%\bin;D`, "String", ""},
		{"a longer one", "String", `D\x;D-old`, `D\x;D-old;D`, "String", ""},
		{"a ; in quotes", "String", `"C:\a;b";C:\c`, `"C:\a;b";C:\c;D`, "String", ""},
		{"named PATH", "ExpandString", `C:\a`, `C:\a;D`, "ExpandString", "PATH"},
	} {
		t.Run("PATH added, "+p.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			base := t.TempDir()
			dir := filepath.Join(base, "bin")
			d := strings.NewReplacer("DUPPER", strings.ToUpper(dir), "D", dir)
			res, _ := install(t, r, psCase{
				PathKind: p.kind, Path: d.Replace(p.path), PathName: p.pathName,
				env: map[string]string{"FLOE_INSTALL_DIR": dir, "FLOE_ADD_TO_PATH": "1", "FLOE_TEST_HOME": base},
			})
			works(t, r, res, dir, host)
			if want := d.Replace(p.after); res.report.Path != want || res.report.PathKind != p.afterKind {
				t.Errorf("the user's Path is %q (%s), want %q (%s):\n%s", res.report.Path, res.report.PathKind, want, p.afterKind, res)
			}
			if p.pathName != "" && res.report.PathName != p.pathName {
				t.Errorf("the value is named %q now, want %q", res.report.PathName, p.pathName)
			}
		})
	}
	t.Run("PATH a drive's root", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		drive := freeDrive(t)
		if out, err := exec.CommandContext(t.Context(), "subst", drive, root).CombinedOutput(); err != nil { //nolint:gosec // G204: subst and a folder of the test
			if os.Getenv("CI") != "" {
				t.Fatalf("subst made no drive: %v %s", err, out)
			}
			t.Skipf("subst made no drive: %v %s", err, out)
		}
		t.Cleanup(func() {
			if err := exec.CommandContext(context.Background(), "subst", drive, "/d").Run(); err != nil { //nolint:gosec // G204: as above
				t.Errorf("subst %s /d: %v", drive, err)
			}
		})
		r := goodRelease(t)
		res, _ := install(t, r, psCase{
			PathKind: "ExpandString", Path: `C:\a;` + drive,
			env: map[string]string{"FLOE_INSTALL_DIR": drive + `\`, "FLOE_ADD_TO_PATH": "1"},
		})
		works(t, r, res, root, host)
		if want := `C:\a;` + drive + `;` + drive + `\`; res.report.Path != want {
			t.Errorf("the user's Path is %q, want %q (D: is that drive's current folder, not its root)", res.report.Path, want)
		}
	})

	// A folder with ; or % cannot be one entry of the user's Path: Windows splits at ; and opens %VAR%.
	for _, name := range []string{"a;b", "50%x%"} {
		t.Run("PATH not one entry, "+name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			dir := filepath.Join(t.TempDir(), name)
			res, _ := install(t, r, psCase{PathKind: "ExpandString", Path: `C:\a`, env: map[string]string{"FLOE_INSTALL_DIR": dir, "x": "X"}})
			works(t, r, res, dir, host)
			if !strings.Contains(res.out, "cannot be written as one PATH entry") || strings.Contains(res.out, "To add it, run:") {
				t.Errorf("want no line to run, and why:\n%s", res)
			}
			res, _ = install(t, r, psCase{PathKind: "ExpandString", Path: `C:\a`, env: map[string]string{"FLOE_INSTALL_DIR": dir, "x": "X", "FLOE_ADD_TO_PATH": "1"}})
			if res.code != 1 || !strings.Contains(res.err, "could not add") || !strings.Contains(res.err, "one PATH entry") {
				t.Errorf("want a failure to add it:\n%s", res)
			}
			if res.report.Path != `C:\a` || res.report.PathKind != "ExpandString" {
				t.Errorf("the user's Path changed to %q (%s)", res.report.Path, res.report.PathKind)
			}
		})
	}
	t.Run("PATH not asked for", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		res, dir := install(t, r, psCase{PathKind: "ExpandString", Path: `%JAVA_HOME%\bin`})
		works(t, r, res, dir, host)
		if res.report.Path != `%JAVA_HOME%\bin` || res.report.PathKind != "ExpandString" {
			t.Errorf("the user's Path changed to %q (%s)", res.report.Path, res.report.PathKind)
		}
		if !strings.Contains(res.out, dir+" is not on your PATH. To add it, run:") {
			t.Errorf("no line to add it:\n%s", res)
		}
	})
	t.Run("PATH on the process's alone", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		dir := filepath.Join(base, "bin")
		res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir, "Path": os.Getenv("Path") + ";" + dir}})
		works(t, r, res, dir, host)
		if strings.Contains(res.out, "your PATH") || res.report.PathKind != "none" {
			t.Errorf("said or wrote of PATH although the process's has it:\n%s", res)
		}
		res, _ = install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir, "Path": os.Getenv("Path") + ";" + dir, "FLOE_ADD_TO_PATH": "1"}})
		if res.report.Path != dir {
			t.Errorf("the user's Path is %q, want %q: the process's PATH alone does not reach a new terminal", res.report.Path, dir)
		}
	})
	t.Run("PATH on the user's alone", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		dir := filepath.Join(base, "bin")
		res, _ := install(t, r, psCase{PathKind: "ExpandString", Path: dir, env: map[string]string{"FLOE_INSTALL_DIR": dir, "FLOE_ADD_TO_PATH": "1"}})
		works(t, r, res, dir, host)
		if !strings.Contains(res.out, dir+" is on your PATH; open a new terminal") || res.report.Path != dir {
			t.Errorf("want only a word to open a new terminal:\n%s", res)
		}
	})
	t.Run("PATH a variable left closed in the process's", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		base := t.TempDir()
		dir := filepath.Join(base, "bin")
		res, _ := install(t, r, psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir, "FLOE_TEST_HOME": base, "Path": os.Getenv("Path") + `;%FLOE_TEST_HOME%\bin`}})
		works(t, r, res, dir, host)
		if !strings.Contains(res.out, "To add it, run:") {
			t.Errorf("Windows does not open %%VAR%% in a process's PATH: want the line:\n%s", res)
		}
	})
	t.Run("PATH a name of the notice is kept", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		res, dir := install(t, r, psCase{Notice: true, env: map[string]string{"FLOE_ADD_TO_PATH": "1"}})
		works(t, r, res, dir, host)
		if !res.report.Notice || res.report.Path != dir {
			t.Errorf("FLOE_INSTALL_PATH_NOTICE removed (%v) or Path %q:\n%s", !res.report.Notice, res.report.Path, res)
		}
	})
	t.Run("PATH not a string", func(t *testing.T) {
		t.Parallel()
		r := goodRelease(t)
		res, dir := install(t, r, psCase{PathKind: "MultiString", Path: `C:\a|C:\b`})
		works(t, r, res, dir, host)
		if !strings.Contains(res.out, "To add it, run:") {
			t.Errorf("no line to add it:\n%s", res)
		}
		r = goodRelease(t)
		res, dir = install(t, r, psCase{PathKind: "MultiString", Path: `C:\a|C:\b`, env: map[string]string{"FLOE_ADD_TO_PATH": "1"}})
		if res.code != 1 || !strings.Contains(res.err, "installed floe 0.1.0") || !strings.Contains(res.err, "could not add") || installed(t, dir) == nil {
			t.Errorf("want floe.exe in place and a failure to add it:\n%s", res)
		}
		if res.report.Path != `C:\a|C:\b` || res.report.PathKind != "MultiString" {
			t.Errorf("the user's Path changed to %q (%s)", res.report.Path, res.report.PathKind)
		}
	})

	// The line shown is run as it is, but `irm <URL>`, read from install.ps1 here, in a new PowerShell elsewhere.
	for _, l := range []struct {
		name, dir, version string
		file               bool // run as a file, from a folder with the same name in it
	}{
		{"plain", "bin", "", false},
		{"a space, a quote, a curly quote, a dollar", "a b'c\u2019d$e", "", false},
		{"pinned", "bin", "0.1.0", false},
		{"as a file", "o'b\u2019c", "", true},
		{"a value built to break out", "x\u2019 + $(New-Item -ItemType File -Path ran) + \u2019y", "", false},
	} {
		t.Run("PATH the line shown, "+l.name, func(t *testing.T) {
			t.Parallel()
			r := goodRelease(t)
			base := t.TempDir()
			dir := filepath.Join(base, l.dir)
			c := psCase{env: map[string]string{"FLOE_INSTALL_DIR": dir, "FLOE_VERSION": l.version}}
			if l.file {
				c.Form = "file"
				c.Script = filepath.Join(dir+"-script", "install.ps1")
				b, err := os.ReadFile(filepath.Join("..", "..", "install.ps1"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(c.Script), 0o700); err != nil {
					t.Fatal(err)
				}
				write(t, c.Script, string(b))
			}
			res, _ := install(t, r, c)
			works(t, r, res, dir, host)
			shown := after(res.out, "To add it, run:\r\n")
			if shown == "" {
				shown = after(res.out, "To add it, run:\n")
			}
			shown = strings.TrimSuffix(shown, "\r")
			if shown == "" {
				t.Fatalf("no line shown:\n%s", res)
			}
			// The seconds to wait the run was given (runPS's) are in the line, as the folder and the version are.
			if !strings.Contains(shown, "$env:FLOE_INSTALL_TIMEOUT = '"+psWait+"'; ") {
				t.Errorf("the line shown is %q, want FLOE_INSTALL_TIMEOUT in it:\n%s", shown, res)
			}
			script, err := filepath.Abs(filepath.Join("..", "..", "install.ps1"))
			if err != nil {
				t.Fatal(err)
			}
			run := strings.Replace(shown, "irm https://raw.githubusercontent.com/sukekyo26/floe/main/install.ps1 | iex",
				"Get-Content -Raw -LiteralPath '"+strings.ReplaceAll(script, "'", "''")+"' | Invoke-Expression", 1)
			if l.file == strings.Contains(shown, "irm ") {
				t.Errorf("the line is not of the form it was run in: %s", shown)
			}
			other := t.TempDir()
			again := runPS(t, shell, other, psCase{Proxy: site(t, goodRelease(t)), Run: run, env: map[string]string{"FLOE_INSTALL_DIR": ""}})
			if again.code != 0 {
				t.Fatalf("the line shown fails:\n%s\n%s", shown, again)
			}
			if again.report.Path != dir {
				t.Errorf("the line shown added %q, want %q:\n%s", again.report.Path, dir, shown)
			}
			if _, err := os.Stat(filepath.Join(other, "w", "ran")); err == nil {
				t.Errorf("a command in the folder's name ran: %s", shown)
			}
		})
	}
}
