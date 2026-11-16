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

	"floe/internal/testfs"
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

	if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}
