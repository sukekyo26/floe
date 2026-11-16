package cli_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/cli"
	"floe/internal/files"
	"floe/internal/testfs"
)

// runIn runs floe with args in dir under the environment env, and returns its exit code, stdout and stderr.
func runIn(dir string, env map[string]string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	getenv := func(key string) string { return env[key] }
	code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir, Getenv: getenv})
	return code, stdout.String(), stderr.String()
}

// The icons are kept in FLOE_ICONS_DIR, else in the default of the system (cli.DefaultPlace), each an absolute path
// whatever the working directory; a relative one is refused, or is none for a base of the system, as the XDG spec
// says.
func TestIconsDir(t *testing.T) {
	t.Parallel()

	t.Run("the places", func(t *testing.T) {
		t.Parallel()
		places(t)
	})
	t.Run("XDG", func(t *testing.T) {
		testfs.UnixOnly(t, "the XDG base directories")
		t.Parallel()
		home, data := t.TempDir(), t.TempDir()
		for _, c := range []struct {
			env  map[string]string
			want string
		}{
			{map[string]string{"XDG_DATA_HOME": data, "HOME": home}, filepath.Join(data, "floe-icons", "aws")},
			{map[string]string{"XDG_DATA_HOME": "~/d", "HOME": home}, filepath.Join(home, ".local", "share", "floe-icons", "aws")},
			{map[string]string{"XDG_DATA_HOME": "d", "HOME": home}, filepath.Join(home, ".local", "share", "floe-icons", "aws")},
		} {
			notIn(t, c.env, c.want)
		}
	})
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{cli.HomeEnv("/h", map[string]string{"FLOE_ICONS_DIR": "~/i"}), "floe: FLOE_ICONS_DIR is ~/i, a relative path; set it to an absolute path\n"},
		{cli.HomeEnv("/h", map[string]string{"FLOE_ICONS_DIR": "i"}), "floe: FLOE_ICONS_DIR is i, a relative path; set it to an absolute path\n"},
		{cli.HomeVars("h"), "floe: there is no directory for icons, as " + cli.PlaceVar("icons") + " is not an absolute path; set FLOE_ICONS_DIR to an absolute path\n"},
		{nil, "floe: there is no directory for icons, as " + cli.PlaceVar("icons") + " is not an absolute path; set FLOE_ICONS_DIR to an absolute path\n"},
	} {
		dirIn(t, c.env, c.want)
	}
	// A setting under a file is told for the setting; Windows reads it as none (W12, iconsUnderAFile).
	t.Run("a setting under a file", func(t *testing.T) {
		testfs.UnixOnly(t, "a path under a file is read as not there on Windows (W12)")
		t.Parallel()
		under := filepath.Join(fileIn(t), "i")
		dirIn(t, cli.HomeEnv(t.TempDir(), map[string]string{"FLOE_ICONS_DIR": under}), "floe: FLOE_ICONS_DIR is "+under+", which cannot be used as a directory (not a directory); make it a directory floe can use, or set FLOE_ICONS_DIR to another directory\n")
	})
}

// fileIn is an empty file in a new temporary directory.
func fileIn(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

// dirIn checks that floe icons search tells want under env, in no directory and in one.
func dirIn(t *testing.T, env map[string]string, want string) {
	t.Helper()
	for _, dir := range []string{"", t.TempDir()} {
		if code, _, stderr := runIn(dir, env, "icons", "search", "aws", "x"); code != 1 || stderr != want {
			t.Errorf("%v in %q: %d, %q, want 1, %q", env, dir, code, stderr, want)
		}
	}
}

// notIn checks that floe icons search under env says the AWS icons are not in the directory want.
func notIn(t *testing.T, env map[string]string, want string) {
	t.Helper()
	_, _, stderr := runIn("", env, "icons", "search", "aws", "x")
	if told := "floe: AWS icons are not in " + want + `; run "floe icons fetch aws" (downloads about 14MB from ` +
		"aws.amazon.com) and try again\n"; stderr != told {
		t.Errorf("%v: %q, want %q", env, stderr, told)
	}
}

// places checks where floe icons search looks for the icons.
func places(t *testing.T) {
	t.Helper()
	icons, home, file := t.TempDir(), t.TempDir(), fileIn(t)
	notIn(t, cli.HomeEnv(home, map[string]string{"FLOE_ICONS_DIR": icons}), filepath.Join(icons, "aws"))
	notIn(t, cli.HomeVars(home), filepath.Join(cli.DefaultPlace(home, "icons"), "aws"))
	// The default under a file holds none.
	notIn(t, cli.HomeVars(file), filepath.Join(cli.DefaultPlace(file, "icons"), "aws"))
}

// floe icons fetch into the default under a file tells why before it reaches the network.
func TestIconsFetchUnderFile(t *testing.T) {
	t.Parallel()

	file := fileIn(t)
	want := cli.DefaultPlace(file, "icons") + ": cannot be written (not a directory): check that its directory exists and can be written to, and that its name is free and not too long\n"
	if code, _, stderr := runIn("", cli.HomeEnv(file, map[string]string{"HTTPS_PROXY": "http://127.0.0.1:1"}), "icons", "fetch", "aws"); code != 1 || stderr != want {
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
		{[]string{"icons", "fetch", "aws", "icons.zip"}, 0, "fetched 1 AWS icons (01022030) into " + filepath.Join(store, "aws") + "\n", ""},
		{[]string{"icons", "fetch", "aws", "icons.zip"}, 0, "AWS icons 01022030 are already in " + filepath.Join(store, "aws") + " (add --force to fetch them again)\n", ""},
		{[]string{"icons", "search", "aws", "lambda"}, 0, "aws/aws-lambda\tCompute\n", ""},
		{[]string{"icons", "fetch", "aws", "page.html"}, 1, "", "page.html: is not a zip file; pass the Icon package zip from https://aws.amazon.com/architecture/icons/: floe icons fetch aws <path-or-url>\n"},
		// Every mistake of the command line is told at once.
		{[]string{"icons", "fetch", "nope", ""}, 1, "", "floe: argument 2 is an empty path; give a zip file of the icons, " +
			"or a URL, and check that a variable written for it is set\n" + `floe: unknown icon set "nope"; use aws, gcp, azure` + "\n"},
	} {
		code, stdout, stderr := runIn(dir, env, c.args...)
		if code != c.code || stdout != c.stdout || stderr != c.stderr {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q, %q", c.args, code, stdout, stderr, c.code, c.stdout, c.stderr)
		}
	}
	t.Run("a zip not there", func(t *testing.T) {
		t.Parallel()
		_, _, gone := files.Read(filepath.Join(dir, "gone.zip"))
		if code, stdout, stderr := runIn(dir, env, "icons", "fetch", "aws", "gone.zip"); code != 1 || stdout != "" || stderr != "gone.zip: "+testfs.Words(t, gone)+"\n" {
			t.Errorf("floe icons fetch aws gone.zip = %d, %q, %q", code, stdout, stderr)
		}
	})

	// The zips are as many as the set's packages, told with the other mistakes of the command line; a set of two
	// takes both.
	code, _, stderr := runIn(dir, env, "icons", "fetch", "aws", "", "b.zip")
	if code != 1 || !strings.Contains(stderr, "argument 2 is an empty path") || !strings.Contains(stderr, "AWS icons come in the Icon package zip alone") {
		t.Errorf("floe icons fetch aws \"\" b.zip = %d, %q; want both mistakes", code, stderr)
	}
	// Azure without its zip is told with the other mistakes too, before the network.
	code, _, stderr = runIn(dir, map[string]string{"FLOE_ICONS_DIR": "relative"}, "icons", "fetch", "azure")
	if code != 1 || !strings.Contains(stderr, "FLOE_ICONS_DIR") || !strings.Contains(stderr, "agreeing to their terms") {
		t.Errorf("FLOE_ICONS_DIR=relative floe icons fetch azure = %d, %q; want both mistakes", code, stderr)
	}
	// The set built in is searched with no directory for icons, and never fetched; a set to fetch still needs one.
	rel := map[string]string{"FLOE_ICONS_DIR": "relative"}
	if code, stdout, stderr := runIn(dir, rel, "icons", "search", "lucide", "user"); code != 0 || !strings.HasPrefix(stdout, "lucide/user\taccount\n") || !strings.Contains(stdout, "lucide/users\t") || stderr != "" {
		t.Errorf("FLOE_ICONS_DIR=relative floe icons search lucide user = %d, %q, %q", code, stdout, stderr)
	}
	if code, _, stderr := runIn(dir, rel, "icons", "search", "aws", "lambda"); code != 1 || !strings.Contains(stderr, "FLOE_ICONS_DIR") {
		t.Errorf("FLOE_ICONS_DIR=relative floe icons search aws lambda = %d, %q; want the directory told", code, stderr)
	}
	if code, stdout, stderr := runIn(dir, rel, "icons", "fetch", "lucide"); code != 1 || stdout != "" ||
		!strings.Contains(stderr, "floe icons search lucide") || strings.Contains(stderr, "FLOE_ICONS_DIR") {
		t.Errorf("FLOE_ICONS_DIR=relative floe icons fetch lucide = %d, %q, %q; want it refused for itself alone", code, stdout, stderr)
	}
	for name, entry := range map[string]string{
		"core.zip": "Unique Icons/GKE/SVG/GKE-512-color.svg", "category.zip": "Category Icons/Storage/SVG/Storage-512-color.svg",
	} {
		var z bytes.Buffer
		zw := zip.NewWriter(&z)
		if _, err := zw.Create(entry); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), z.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code, stdout, stderr := runIn(dir, env, "icons", "fetch", "gcp", "core.zip", "category.zip"); code != 0 || !strings.HasPrefix(stdout, "fetched 2 Google Cloud icons") {
		t.Errorf("floe icons fetch gcp core.zip category.zip = %d, %q, %q", code, stdout, stderr)
	}
	if code, stdout, _ := runIn(dir, env, "icons", "search", "gcp", "gke"); code != 0 || stdout != "gcp/gke\tCore product\n" {
		t.Errorf("floe icons search gcp gke = %d, %q", code, stdout)
	}
}

// floe icons search takes a set and a word or more: fewer is told in one line, an unknown set only with words, and a
// set not fetched with how to fetch it.
func TestIconsSearchArgs(t *testing.T) {
	t.Parallel()

	env := map[string]string{"FLOE_ICONS_DIR": t.TempDir()}
	words := "name the icon set and a word to search for"
	for _, c := range []struct {
		args []string
		want string // what the one line of stderr holds
	}{
		{[]string{"icons", "search"}, words},
		{[]string{"icons", "search", "storage"}, words},
		{[]string{"icons", "search", "gcp"}, words},
		{[]string{"icons", "search", "ibm", "x"}, `unknown icon set "ibm"`},
		{[]string{"icons", "search", "gcp", "storage"}, `Google Cloud icons are not in`},
		{[]string{"icons", "search", "azure", "vm"}, `agree to their terms`},
	} {
		code, stdout, stderr := runIn("", env, c.args...)
		if code != 1 || stdout != "" || strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, c.want) {
			t.Errorf("floe %v = %d, %q, %q; want 1 and one line holding %q", c.args, code, stdout, stderr, c.want)
		}
	}
}
