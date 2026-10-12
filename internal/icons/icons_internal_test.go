package icons

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// When the new icons cannot be moved into place, the previous ones are put back.
func TestFetchRestoresPrevious(t *testing.T) { //nolint:paralleltest // replaces rename
	root := t.TempDir()
	set := New(root)
	dir := filepath.Join(root, "aws")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	previous := `{"version": "01012020", "source": "before", "icons": {}}`
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(previous), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "Icon-package.zip")
	if err := os.WriteFile(src, zipOf(t, "Architecture-Group-Icons_01022030/Region_32.svg"), 0o600); err != nil {
		t.Fatal(err)
	}
	rename = func(from, to string) error {
		if to == dir {
			return errors.New("injected")
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { rename = os.Rename })

	_, err := set.Fetch(context.Background(), Default, src, true)
	if err == nil || !strings.Contains(err.Error(), "injected") {
		t.Errorf("Fetch = %v, want the injected failure", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "index.json")); err != nil || string(b) != previous {
		t.Errorf("index.json after the failure = %s, %v; want the previous one", b, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 {
		t.Errorf("the icons' root holds %v, %v; want aws alone", entries, err)
	}
}

func TestPackageURL(t *testing.T) {
	t.Parallel()

	page := `<a href="https://d1.awsstatic.com/x/Icon-package_07312026.abc.zip">Icon package</a> ` +
		`<a href="https://d1.awsstatic.com/x/Microsoft-PPTx-toolkits_07312026.zip">`
	if got, want := aws.link.FindString(page), "https://d1.awsstatic.com/x/Icon-package_07312026.abc.zip"; got != want {
		t.Errorf("package URL = %s, want %s", got, want)
	}
}

// A name part of the zip cannot hold a slash, so no entry writes outside the icons' directory.
func TestIconOfStaysInside(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"Architecture-Service-Icons_01022030/Arch_Compute/48/Arch_../../../x_48.svg",
		"Architecture-Group-Icons_01022030/../../x_32.svg",
		"Resource-Icons_01022030/Res_Compute/Res_../x_48.svg",
	} {
		if icon, ok := awsIconOf(path); ok {
			t.Errorf("awsIconOf(%s) keeps %s", path, icon.name)
		}
	}
}

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range names {
		if _, err := zw.Create(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
