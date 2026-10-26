package icons

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

func told(err error) string {
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

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
	if err := os.WriteFile(src, zipOf(t, map[string]int{"Architecture-Group-Icons_01022030/Region_32.svg": 1}), 0o600); err != nil {
		t.Fatal(err)
	}
	rename = func(from, to string) error {
		if to == dir {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: errors.New("injected")}
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { rename = os.Rename })

	_, err := set.Fetch(context.Background(), "aws", Source{Name: src, At: src}, true)
	if got := told(err); !strings.Contains(got, "cannot be written (injected)") {
		t.Errorf("Fetch = %s, want the injected failure", got)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "index.json")); err != nil || string(b) != previous {
		t.Errorf("index.json after the failure = %s, %v; want the previous one", b, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 {
		t.Errorf("the icons' root holds %v, %v; want aws alone", entries, err)
	}
}

// When neither the new icons nor the previous ones can be moved into place, the mistake says where the previous
// ones are.
func TestFetchLeavesPrevious(t *testing.T) { //nolint:paralleltest // replaces rename
	root := t.TempDir()
	dir := filepath.Join(root, "aws")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "Icon-package.zip")
	if err := os.WriteFile(src, zipOf(t, map[string]int{"Architecture-Group-Icons_01022030/Region_32.svg": 1}), 0o600); err != nil {
		t.Fatal(err)
	}
	var old string
	rename = func(from, to string) error {
		old = from + "-old"
		// Something takes the set's place, so the previous icons cannot go back either.
		if err := os.MkdirAll(filepath.Join(to, "x"), 0o700); err != nil {
			return err
		}
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: errors.New("injected")}
	}
	t.Cleanup(func() { rename = os.Rename })

	_, err := New(root).Fetch(context.Background(), "aws", Source{Name: src, At: src}, true)
	want := dir + ": cannot be written (injected), and the previous icons are left in " + old +
		": move them back here, or fetch the icons again"
	if told(err) != want {
		t.Errorf("Fetch = %s, want %s", told(err), want)
	}
}

// The package is the first <a> whose href, resolved against the page, is one of the set's packages, as HTML
// reads it: its names in any case, its character references decoded.
func TestPackageURL(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		page, want string
	}{
		{
			`<a href="https://d1.awsstatic.com/x/Microsoft-PPTx-toolkits_07312026.zip">` +
				`<a href="https://d1.awsstatic.com/x/Icon-package_07312026.abc.zip">Icon package</a>`,
			"https://d1.awsstatic.com/x/Icon-package_07312026.abc.zip",
		},
		{`<A HREF='https://d1.awsstatic.com/x/Icon-package_07312026.zip'>`, "https://d1.awsstatic.com/x/Icon-package_07312026.zip"},
		{`<a href="https://d1.awsstatic.com/a&amp;b/Icon-package_07312026.zip">`, "https://d1.awsstatic.com/a&b/Icon-package_07312026.zip"},
		{`<a href="/x/Icon-package_07312026.zip">`, "https://aws.amazon.com/x/Icon-package_07312026.zip"},
		{`<script>{"url":"https://d1.awsstatic.com/x/Icon-package_07312026.zip"}</script>`, ""},
		{`<link href="https://d1.awsstatic.com/x/Icon-package_07312026.zip">`, ""},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, c.page) //nolint:errcheck // the test reads what arrives
		}))
		k := aws
		k.page = srv.URL + "/architecture/icons/"
		// The test's server is http: the set's pattern is let take it.
		k.link = regexp.MustCompile(strings.Replace(aws.link.String(), "^https://", "^https?://", 1))
		got, err := k.packageURL(context.Background())
		srv.Close()
		want := strings.Replace(c.want, "https://aws.amazon.com", srv.URL, 1)
		if c.want == "" {
			if wantErr := k.page + `: no link to the icon package is on this page; download the zip there and run: ` +
				`floe icons fetch aws <path-or-url>`; told(err) != wantErr {
				t.Errorf("page %s = %q, %s; want %s", c.page, got, told(err), wantErr)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("page %s = %q, %v; want %s", c.page, got, err, want)
		}
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

// zipOf is a zip of the entries, each of its size in bytes.
func zipOf(t *testing.T, entries map[string]int) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, size := range entries {
		w, err := zw.Create(name)
		if err == nil {
			_, err = w.Write(bytes.Repeat([]byte("x"), size))
		}
		if err != nil {
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

// A download gives up on a server that does not answer or stops sending, and refuses one past its limit.
func TestDownloadLimits(t *testing.T) { //nolint:paralleltest // changes the limits
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
	tooLarge := ": the download is over 0MB, far larger than an icon package; check that it is the right URL"
	for path, want := range map[string]string{
		"/silent":        ": cannot be downloaded (no response in 100ms); check the network, or download it in a browser and pass its path",
		"/stalls":        ": cannot be downloaded (no data for 100ms); check the network, or download it in a browser and pass its path",
		"/large":         tooLarge,
		"/large-unsized": tooLarge,
	} {
		if _, err := download(ctx, srv.URL+path, "icons_download"); told(err) != srv.URL+path+want {
			t.Errorf("download %s = %s, want %s", path, told(err), srv.URL+path+want)
		}
	}
	if b, err := download(ctx, srv.URL+"/slow", "icons_download"); err != nil || len(b) != 40 {
		t.Errorf("download /slow = %d bytes, %v; want all 40", len(b), err)
	}
}

// What a package writes is capped: an icon, the number of icons and the icons in all, each told as its own.
func TestExtractLimits(t *testing.T) { //nolint:paralleltest // changes the limits
	set(t, &maxIcon, 10)
	set(t, &maxIcons, 2)
	set(t, &maxPackage, 25)
	icon := func(name string) string { return "Architecture-Group-Icons_01022030/" + name + "_32.svg" }
	for _, c := range []struct {
		entries map[string]int
		want    string
	}{
		{map[string]int{icon("A"): 11}, "p.zip: " + icon("A") + " in it is over 0MB, far larger than an icon; check that it is the right file"},
		{map[string]int{icon("A"): 1, icon("B"): 1, icon("C"): 1}, "p.zip: holds over 2 icons, far more than an icon package; check that it is the right file"},
		{map[string]int{icon("A"): 10, icon("B"): 10}, ""},
	} {
		b := zipOf(t, c.entries)
		_, err := aws.extract(bytes.NewReader(b), int64(len(b)), Source{Name: "p.zip", At: "/p.zip"}, t.TempDir())
		if (err != nil || c.want != "") && told(err) != c.want {
			t.Errorf("extract %v = %s, want %q", c.entries, told(err), c.want)
		}
	}
	set(t, &maxPackage, 15)
	b := zipOf(t, map[string]int{icon("A"): 10, icon("B"): 10})
	_, err := aws.extract(bytes.NewReader(b), int64(len(b)), Source{Name: "p.zip", At: "/p.zip"}, t.TempDir())
	if want := "p.zip: the icons in it come to over 0MB, far more than an icon package; check that it is the right file"; told(err) != want {
		t.Errorf("extract of icons past the limit in all = %s, want %s", told(err), want)
	}
}

// The set's page that cannot be downloaded asks for the package's zip, not the page; a URL net/http cannot
// read says so, without the URL twice.
func TestDownloadMistakes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	k := aws
	k.page = srv.URL + "/icons/"
	_, err := k.packageURL(context.Background())
	if want := k.page + ": cannot be downloaded (HTTP 404); check the network, or download the package's zip from this page " +
		"and pass its path"; told(err) != want {
		t.Errorf("a page not there = %s, want %s", told(err), want)
	}
	_, err = download(context.Background(), "http://%zz", "icons_download")
	if want := `http://%zz: is not a URL floe can download (invalid URL escape "%zz"); check how it is written`; told(err) != want {
		t.Errorf("a URL of no form = %s, want %s", told(err), want)
	}
	_, err = download(context.Background(), "http://127.0.0.1:1/x.zip", "icons_download")
	if got := told(err); strings.Count(got, "http://127.0.0.1:1/x.zip") != 1 || strings.Contains(got, "Get ") {
		t.Errorf("a URL nothing answers = %s, want the URL once, without net/http's request before its words", got)
	}
}

// A URL is told by its scheme, in any case; its version, by the last name of its path.
func TestURLs(t *testing.T) {
	t.Parallel()

	for source, want := range map[string]bool{
		"https://x/a.zip": true, "HTTPS://x/a.zip": true, "http://x/a.zip": true,
		"/tmp/a.zip": false, "a.zip": false, "ftp://x/a.zip": false, "c:a.zip": false,
	} {
		if IsURL(source) != want {
			t.Errorf("IsURL(%s) = %v", source, !want)
		}
	}
	for at, want := range map[string]bool{
		"https://x/Icon-package_01022030.abc.zip":            true,
		"https://x/a.zip?v=Icon-package_01022030.zip":        false,
		"https://x/Icon-package_01022030/other.zip":          false,
		"https://x/Icon-package_01022030.zip#Icon-package_1": true,
		"/tmp/Icon-package_01022030.zip":                     false,
	} {
		if aws.tells(at, "01022030") != want {
			t.Errorf("tells(%s) = %v", at, !want)
		}
	}
}
