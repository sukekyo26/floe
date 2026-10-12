package icons

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	_, err := set.Fetch(context.Background(), "aws", src, true)
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

// set changes *v to val for the test. The tests that do are not parallel.
func set[T any](t *testing.T, v *T, val T) {
	t.Helper()
	old := *v
	*v = val
	t.Cleanup(func() { *v = old })
}

// A download gives up on a server that does not answer or stops sending, and a package or an icon
// past its limit is refused.
func TestReadLimits(t *testing.T) { //nolint:paralleltest // changes the limits
	set(t, &headerTimeout, 100*time.Millisecond)
	set(t, &stallTimeout, 100*time.Millisecond)
	set(t, &maxPackage, 64)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/silent":
			<-release
		case "/stalls":
			_, _ = w.Write([]byte("PK"))              //nolint:errcheck // the test reads what arrives
			_ = http.NewResponseController(w).Flush() //nolint:errcheck // as above
			<-release
		case "/slow":
			// 40 bytes in 4 pieces, 60ms apart: slower than the stall limit in all, never stalled.
			for range 4 {
				_, _ = w.Write(bytes.Repeat([]byte("x"), 10)) //nolint:errcheck // as above
				_ = http.NewResponseController(w).Flush()     //nolint:errcheck // as above
				time.Sleep(60 * time.Millisecond)
			}
		case "/large":
			w.Header().Set("Content-Length", "65")
			_, _ = w.Write(bytes.Repeat([]byte("x"), 65)) //nolint:errcheck // as above
		case "/large-unsized":
			for range 2 {
				_, _ = w.Write(bytes.Repeat([]byte("x"), 40)) //nolint:errcheck // as above
				_ = http.NewResponseController(w).Flush()     //nolint:errcheck // as above
			}
		}
	}))
	// Cleanups run last first: the handlers are let go before the server waits for them.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	ctx := context.Background()
	for path, want := range map[string]string{
		"/silent":        "timeout awaiting response headers",
		"/stalls":        "(no data for 100ms)",
		"/large":         "is over 0MB",
		"/large-unsized": "is over 0MB",
	} {
		if _, err := read(ctx, srv.URL+path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("read %s = %v, want %s", path, err, want)
		}
	}
	if b, err := read(ctx, srv.URL+"/slow"); err != nil || len(b) != 40 {
		t.Errorf("read /slow = %d bytes, %v; want all 40", len(b), err)
	}
	file := filepath.Join(t.TempDir(), "large.zip")
	if err := os.WriteFile(file, bytes.Repeat([]byte("x"), 65), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := read(ctx, file); err == nil || !strings.Contains(err.Error(), "is over 0MB") {
		t.Errorf("read a large file = %v", err)
	}

	set(t, &maxPackage, 256<<20)
	set(t, &maxIcon, 10)
	zipFile := filepath.Join(t.TempDir(), "Icon-package.zip")
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	w, err := zw.Create("Architecture-Group-Icons_01022030/Region_32.svg")
	if err == nil {
		_, err = w.Write(bytes.Repeat([]byte("x"), 11))
	}
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(zipFile, zb.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = New(t.TempDir()).Fetch(ctx, "aws", zipFile, false)
	if err == nil || !strings.Contains(err.Error(), "(larger than any icon); download the zip again") {
		t.Errorf("Fetch with an icon past the limit = %v", err)
	}
}

func TestNear(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, word string
		want       bool
	}{
		{"aws-lambda", "lambda", true},
		{"aws-lambda", "lamda", true},         // 1 edit
		{"aws-lambda", "lmda", false},         // 2 edits for a short word
		{"amazon-dynamodb", "dynamdbo", true}, // 2 edits for a long word
		{"amazon-ec2", "ec3", false},          // a short word must be in the name
	} {
		if got := near(c.name, c.word); got != c.want {
			t.Errorf("near(%s, %s) = %v, want %v", c.name, c.word, got, c.want)
		}
	}
}
