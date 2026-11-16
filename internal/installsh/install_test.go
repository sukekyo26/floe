package installsh_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The fakes stand for the network and the machine: curl serves $FAKE_API for the API and
// $FAKE_ASSETS/<tag>/<name> for downloads, uname prints $FAKE_OS and $FAKE_ARCH. Neither reads
// stdin, which holds install.sh.
const (
	fakeCurl = `#!/bin/sh
out= url=
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    https://*) url=$1 ;;
  esac
  shift
done
case "$url" in
  https://api.github.com/repos/sukekyo26/floe/releases/latest)
    [ -f "$FAKE_API" ] || { echo 'curl: (22) The requested URL returned error: 404' >&2; exit 22; }
    cat "$FAKE_API" ;;
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

	tests := []struct {
		name, os, arch, api, assets, version, dir string
		shasum                                    bool   // only shasum on PATH, not sha256sum
		readOnly                                  bool   // the install directory is there but not writable
		want                                      string // the asset installed, or "" when it fails
		says                                      []string
	}{
		{name: "latest", os: "Linux", arch: "x86_64", api: pretty, want: "floe-linux-amd64"},
		{name: "latest one line", os: "Darwin", arch: "arm64", api: compact, want: "floe-darwin-arm64"},
		{name: "pinned", os: "Linux", arch: "aarch64", version: "0.1.0", want: "floe-linux-arm64"},
		{name: "pinned with v", os: "Linux", arch: "amd64", version: "v0.1.0", want: "floe-linux-amd64"},
		{name: "shasum", os: "Darwin", arch: "x86_64", version: "0.1.0", shasum: true, want: "floe-darwin-amd64"},
		{name: "checksum mismatch", os: "Linux", arch: "x86_64", version: "0.1.0", assets: bad, says: []string{"checksum mismatch"}},
		{name: "no release", os: "Linux", arch: "x86_64", says: []string{"FLOE_VERSION", "try again later"}},
		{name: "no such version", os: "Linux", arch: "x86_64", version: "0.9.9", says: []string{"download failed", "v0.9.9/floe-linux-amd64"}},
		{name: "arch", os: "Linux", arch: "armv7l", says: []string{"armv7l", "amd64 and arm64"}},
		{name: "os", os: "FreeBSD", arch: "amd64", says: []string{"freebsd", "linux and darwin"}},
		{name: "dir not writable", os: "Linux", arch: "x86_64", version: "0.1.0", dir: filepath.Join(notDir, "bin"), says: []string{"FLOE_INSTALL_DIR"}},
		{name: "dir read-only", os: "Linux", arch: "x86_64", version: "0.1.0", readOnly: true, says: []string{"FLOE_INSTALL_DIR"}},
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
			cmd := exec.CommandContext(t.Context(), "sh")
			cmd.Stdin = bytes.NewReader(script)
			cmd.Env = []string{
				"PATH=" + path, "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(),
				"FLOE_INSTALL_DIR=" + dir, "FLOE_VERSION=" + tt.version,
				"FAKE_OS=" + tt.os, "FAKE_ARCH=" + tt.arch, "FAKE_API=" + tt.api, "FAKE_ASSETS=" + from,
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			for _, s := range tt.says {
				if !strings.Contains(stderr.String(), s) {
					t.Errorf("stderr does not say %q:\n%s", s, stderr.String())
				}
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

// TestPathNote is what install.sh says of a directory that is not on PATH. The line it shows to a zsh or a Linux
// bash is run here, in sh, and the file it wrote is read as that shell reads it.
// LIMIT: the line shown to fish is compared letter by letter, never run: CI has no fish. It was run once by hand, in
// fish 4.2.1, for the directories below.
func TestPathNote(t *testing.T) {
	t.Parallel()

	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := tools(t, filepath.Join(root, "tools"), "sha256sum")
	assets := filepath.Join(root, "assets")
	release(t, assets, "")

	const (
		run    = "run"    // a command to paste, which is run here
		fish   = "fish"   // fish's command, compared
		line   = "line"   // the line alone, no file named
		words  = "words"  // no command and no line
		cannot = "cannot" // the directory cannot be on PATH
		none   = "none"   // it is on PATH: nothing is said
	)
	tests := []struct {
		name, os, shell string
		dir             string // under the case's own directory when relative to it: H is its home, W where it runs
		noHome, noShell bool
		onPath          string // added to PATH, H and W read as in dir
		cdpath          bool   // CDPATH names a directory that holds a bin too
		before          string // what the startup file holds already
		want            string
		file            string // the startup file a run command writes, under the home
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
		{name: "backslashes", os: "Linux", shell: "/bin/bash", dir: `W/back\slash\n`, want: run, file: ".bashrc"},
		{name: "a command in backquotes", os: "Linux", shell: "/bin/zsh", dir: "W/a`touch ran`b", want: run, file: ".zshrc"},
		{name: "a command in $()", os: "Linux", shell: "/bin/bash", dir: "W/$(touch ran)", want: run, file: ".bashrc"},
		{name: "relative", os: "Linux", shell: "/bin/bash", dir: "bin", want: run, file: ".bashrc"},
		{name: "relative, CDPATH set", os: "Linux", shell: "/bin/bash", dir: "bin", cdpath: true, want: run, file: ".bashrc"},
		{name: "fish", os: "Linux", shell: "/usr/bin/fish", dir: "W/o'brien\\x", want: fish},
		{name: "bash on macOS", os: "Darwin", shell: "/bin/bash", dir: "H/.local/bin", want: line, inHome: true},
		{name: "dash", os: "Linux", shell: "/bin/dash", dir: "W/a b", want: line},
		{name: "ksh", os: "Linux", shell: "/bin/ksh", dir: "W/bin", want: line},
		{name: "tcsh", os: "Linux", shell: "/bin/tcsh", dir: "W/bin", want: words},
		{name: "SHELL empty", os: "Linux", dir: "W/bin", want: words},
		{name: "no SHELL", os: "Linux", noShell: true, dir: "W/bin", want: words},
		{name: "no HOME", os: "Linux", shell: "/bin/zsh", noHome: true, dir: "W/bin", want: run, file: ".zshrc"},
		{name: "a colon", os: "Linux", shell: "/bin/zsh", dir: "W/a:b", want: cannot},
		{name: "a line break", os: "Linux", shell: "/bin/zsh", dir: "W/a\nb", want: cannot},
		{name: "on PATH", os: "Linux", shell: "/bin/zsh", dir: "W/bin", onPath: "W/bin", want: none},
		{name: "on PATH, given relative", os: "Linux", shell: "/bin/zsh", dir: "bin", onPath: "W/bin", want: none},
		{name: "on PATH with a slash", os: "Linux", shell: "/bin/zsh", dir: "W/bin", onPath: "W/bin/", want: none},
		{name: "a longer one on PATH", os: "Linux", shell: "/bin/zsh", dir: "W/bin", onPath: "W/bin2", want: run, file: ".zshrc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
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

			cmd := exec.CommandContext(t.Context(), "sh")
			cmd.Stdin = bytes.NewReader(script)
			cmd.Dir = work
			cmd.Env = []string{
				"TMPDIR=" + t.TempDir(), "FLOE_INSTALL_DIR=" + dir, "FLOE_VERSION=0.1.0",
				"FAKE_OS=" + tt.os, "FAKE_ARCH=x86_64", "FAKE_ASSETS=" + assets,
			}
			if tt.onPath == "" {
				cmd.Env = append(cmd.Env, "PATH="+path)
			} else {
				cmd.Env = append(cmd.Env, "PATH="+path+":"+at(tt.onPath))
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
			shown := after(said, "To add it, run:\n")
			put := after(said, "put this line in your shell's startup file:\n")
			for what, has := range map[string]bool{
				"a command": shown != "", "a line": put != "",
				"that it cannot be on PATH": strings.Contains(said, "cannot be a PATH entry"),
				"only words":                strings.Contains(said, "add it to the PATH your shell starts with"),
			} {
				want := map[string]string{"a command": run + fish, "a line": line, "that it cannot be on PATH": cannot, "only words": words}[what]
				if has != strings.Contains(want, tt.want) {
					t.Errorf("install.sh says %s: %v, for a case of %q:\n%s", what, has, tt.want, said)
				}
			}
			if t.Failed() {
				return
			}
			if again := strings.Contains(said, "then open a new terminal"); again != (tt.want == run || tt.want == line) {
				t.Errorf("install.sh says to open a new terminal: %v:\n%s", again, said)
			}
			if tt.want != none && tt.want != cannot && !strings.Contains(said, abs+" is not on your PATH") {
				t.Errorf("install.sh does not name %q as not on PATH:\n%s", abs, said)
			}

			switch tt.want {
			case fish:
				quoted := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(abs)
				if want := "fish_add_path '" + quoted + "'"; shown != want {
					t.Errorf("shown %q, want %q", shown, want)
				}
			case line:
				if strings.Contains(said, ">>") {
					t.Errorf("install.sh names a file to a shell whose file it cannot know:\n%s", said)
				}
				rc := filepath.Join(base, "rc")
				write(t, rc, put+"\n")
				first(t, rc, home, abs, tt.inHome)
			case run:
				rc := filepath.Join(home, tt.file)
				if tt.before != "" {
					write(t, rc, tt.before)
				}
				if !strings.HasSuffix(shown, " >> "+map[string]string{".zshrc": `"${ZDOTDIR:-$HOME}/.zshrc"`, ".bashrc": "~/.bashrc"}[tt.file]) {
					t.Errorf("the command writes another file than %s: %s", tt.file, shown)
				}
				paste := exec.CommandContext(t.Context(), "sh", "-c", shown) //nolint:gosec // G204: the command install.sh shows, which is what is tested
				paste.Dir = work
				paste.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
				if out, err := paste.CombinedOutput(); err != nil {
					t.Fatalf("the command shown fails: %v\n%s\n%s", err, shown, out)
				}
				got, err := os.ReadFile(rc)
				if err != nil {
					t.Fatal(err)
				}
				if tt.before != "" && !strings.HasPrefix(string(got), tt.before+"\n") {
					t.Errorf("the file's own last line changed: %q", got)
				}
				first(t, rc, home, abs, tt.inHome)
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

// first reads the startup file rc as a shell does and fails t unless PATH then starts with dir. With inHome the file
// names dir by $HOME, so that another home gives another directory.
func first(t *testing.T, rc, home, dir string, inHome bool) {
	t.Helper()

	read := func(home string) string {
		cmd := exec.CommandContext(t.Context(), "sh", "-c", `. "$1" && printf %s "$PATH"`, "sh", rc)
		cmd.Env = []string{"PATH=/rest", "HOME=" + home}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("reading %s: %v\n%s", rc, err, out)
		}
		return string(out)
	}
	if got, want := read(home), dir+":/rest"; got != want {
		t.Errorf("PATH is %q after the file is read, want %q", got, want)
	}
	other := read("/another")
	if moved := strings.HasPrefix(other, "/another/"); moved != inHome {
		t.Errorf("with another home PATH is %q; the file names the directory by $HOME: want %v", other, inHome)
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
		if err := os.Symlink(at, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
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

	if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}
