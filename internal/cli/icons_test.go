package cli_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
)

// runIn runs floe with args in dir under the environment env, and returns its exit code, stdout and stderr.
func runIn(dir string, env map[string]string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	getenv := func(key string) string { return env[key] }
	code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir, Getenv: getenv})
	return code, stdout.String(), stderr.String()
}

// The icons are kept in FLOE_ICONS_DIR, else under XDG_DATA_HOME, else under HOME, each an absolute path
// whatever the working directory; a relative one is refused, or is none for XDG_DATA_HOME, as its spec says.
func TestIconsDir(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"FLOE_ICONS_DIR": "/i", "XDG_DATA_HOME": "/d", "HOME": "/h"}, "/i/aws"},
		{map[string]string{"XDG_DATA_HOME": "/d", "HOME": "/h"}, "/d/floe-icons/aws"},
		{map[string]string{"XDG_DATA_HOME": "~/d", "HOME": "/h"}, "/h/.local/share/floe-icons/aws"},
		{map[string]string{"XDG_DATA_HOME": "d", "HOME": "/h"}, "/h/.local/share/floe-icons/aws"},
		{map[string]string{"HOME": "/h"}, "/h/.local/share/floe-icons/aws"},
		// The default under a file holds none.
		{map[string]string{"HOME": "/dev/null"}, "/dev/null/.local/share/floe-icons/aws"},
	} {
		_, _, stderr := runIn("", c.env, "icons", "search", "x")
		want := "floe: AWS icons are not in " + c.want + `; run "floe icons fetch aws" (downloads about 14MB from ` +
			"aws.amazon.com) and try again\n"
		if stderr != want {
			t.Errorf("%v: %q, want %q", c.env, stderr, want)
		}
	}
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"FLOE_ICONS_DIR": "~/i", "HOME": "/h"}, "floe: FLOE_ICONS_DIR is ~/i, a relative path; set it to an absolute path\n"},
		{map[string]string{"FLOE_ICONS_DIR": "i", "HOME": "/h"}, "floe: FLOE_ICONS_DIR is i, a relative path; set it to an absolute path\n"},
		{map[string]string{"HOME": "h"}, "floe: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path\n"},
		{nil, "floe: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path\n"},
		// A setting under a file is told for the setting.
		{map[string]string{"FLOE_ICONS_DIR": "/dev/null/i", "HOME": "/h"}, "floe: FLOE_ICONS_DIR is /dev/null/i, which cannot be used as a directory (not a directory); make it a directory floe can use, or set FLOE_ICONS_DIR to another directory\n"},
	} {
		for _, dir := range []string{"", t.TempDir()} {
			if code, _, stderr := runIn(dir, c.env, "icons", "search", "x"); code != 1 || stderr != c.want {
				t.Errorf("%v in %q: %d, %q, want 1, %q", c.env, dir, code, stderr, c.want)
			}
		}
	}
}

// floe icons fetch into the default under a file tells why before it reaches the network.
func TestIconsFetchUnderFile(t *testing.T) {
	t.Parallel()

	const want = "/dev/null/.local/share/floe-icons: cannot be written (not a directory): check that its directory exists and can be written to, and that its name is free and not too long\n"
	if code, _, stderr := runIn("", map[string]string{"HOME": "/dev/null", "HTTPS_PROXY": "http://127.0.0.1:1"}, "icons", "fetch", "aws"); code != 1 || stderr != want {
		t.Errorf("floe icons fetch aws = %d, %q, want 1, %q", code, stderr, want)
	}
}

// floe icons fetch keeps a set from a zip given by its path, as written, and floe icons search finds its icons.
func TestIconsFetchAndSearch(t *testing.T) {
	t.Parallel()

	dir, store := t.TempDir(), t.TempDir()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	if _, err := zw.Create("Architecture-Service-Icons_01022030/Arch_Compute/48/Arch_AWS-Lambda_48.svg"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "icons.zip"), b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"FLOE_ICONS_DIR": store}
	for _, c := range []struct {
		args           []string
		code           int
		stdout, stderr string
	}{
		{[]string{"icons", "fetch", "aws", "icons.zip"}, 0, "fetched 1 AWS icons (01022030) into " + store + "/aws\n", ""},
		{[]string{"icons", "fetch", "aws", "icons.zip"}, 0, "AWS icons 01022030 are already in " + store + "/aws (add --force to fetch them again)\n", ""},
		{[]string{"icons", "search", "lambda"}, 0, "aws/aws-lambda\tCompute\n", ""},
		{[]string{"icons", "fetch", "aws", "page.html"}, 1, "", "page.html: is not a zip file; pass the zip of the package on https://aws.amazon.com/architecture/icons/\n"},
		{[]string{"icons", "fetch", "aws", "gone.zip"}, 1, "", "gone.zip: no such file or directory\n"},
		// Every mistake of the command line is told at once.
		{[]string{"icons", "fetch", "gcp", ""}, 1, "", "floe: argument 2 is an empty path; give a zip file of the icons, " +
			"or a URL, and check that a variable written for it is set\n" + `floe: unknown icon set "gcp"; use aws` + "\n"},
	} {
		code, stdout, stderr := runIn(dir, env, c.args...)
		if code != c.code || stdout != c.stdout || stderr != c.stderr {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q, %q", c.args, code, stdout, stderr, c.code, c.stdout, c.stderr)
		}
	}
}
