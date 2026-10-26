package icons_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
)

// packageFiles stand in for the Icon package: what is kept, a service in two categories, sizes and
// folders that are not kept, a resource with only a dark variant, one whose dark variant comes first, and
// macOS's resource forks.
var packageFiles = []string{
	"Architecture-Service-Icons_01022030/Arch_Artificial-Intelligence/48/Arch_Amazon-Bedrock_48.svg",
	"Architecture-Service-Icons_01022030/Arch_Compute/48/Arch_AWS-Lambda_48.svg",
	"Architecture-Service-Icons_01022030/Arch_Compute/32/Arch_AWS-Lambda_32.svg",
	"Architecture-Service-Icons_01022030/Arch_Management-Tools/48/Arch_AWS-Lambda_48.svg",
	"Architecture-Group-Icons_01022030/AWS-Cloud-logo_32.svg",
	"Architecture-Group-Icons_01022030/AWS-Cloud-logo_32_Dark.svg",
	"Architecture-Group-Icons_01022030/Region_32.svg",
	"Resource-Icons_01022030/Res_General-Icons/Res_48_Light/Res_Client_48_Light.svg",
	"Resource-Icons_01022030/Res_General-Icons/Res_48_Dark/Res_Client_48_Dark.svg",
	"Resource-Icons_01022030/Res_General-Icons/Res_48_Dark/Res_Night_48_Dark.svg",
	"Resource-Icons_01022030/Res_General-Icons/Res_48_Dark/Res_Server_48_Dark.svg",
	"Resource-Icons_01022030/Res_General-Icons/Res_48_Light/Res_Server_48_Light.svg",
	"Resource-Icons_01022030/Res_Compute/Res_AWS-Lambda_Lambda-Function_48.svg",
	"Category-Icons_01022030/Arch-Category_Compute_48.svg",
	"__MACOSX/Architecture-Service-Icons_01022030/Arch_Compute/48/._Arch_AWS-Lambda_48.svg",
}

// writeZip writes a zip of files, each an SVG holding its own path, and returns its path.
func writeZip(t *testing.T, name string, files []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><title>` + name + "</title></svg>\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

const page = "https://aws.amazon.com/architecture/icons/"

// told is err as the command line tells it.
func told(err error) string {
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

func local(path string) icons.Source {
	return icons.Source{Name: filepath.Base(path), At: path}
}

func fetch(t *testing.T, set *icons.Store, from icons.Source, force bool) icons.Fetched {
	t.Helper()
	got, err := set.Fetch(context.Background(), "aws", from, force)
	if err != nil {
		t.Fatal(told(err))
	}
	return got
}

func TestFetchAndSearch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	set := icons.New(root)
	dir := filepath.Join(root, "aws")
	// Before a fetch, a search and a lookup say how to fetch.
	notFetched := "floe: AWS icons are not in " + dir + `; run "floe icons fetch aws" (downloads about 14MB from aws.amazon.com) and try again`
	_, err := set.Search([]string{"lambda"})
	if got := told(err); got != notFetched {
		t.Errorf("Search before a fetch = %s, want %s", got, notFetched)
	}
	_, err = set.Lookup("aws/aws-lambda")
	if got := told(err); got != notFetched {
		t.Errorf("Lookup before a fetch = %s, want %s", got, notFetched)
	}

	zipFile := local(writeZip(t, "Icon-package.zip", packageFiles))
	got := fetch(t, set, zipFile, false)
	if want := (icons.Fetched{Title: "AWS", Dir: dir, Version: "01022030", Count: 8, Already: false}); got != want {
		t.Errorf("Fetch = %+v, want %+v", got, want)
	}
	// The same version again is kept, unless forced.
	if got := fetch(t, set, zipFile, false); !got.Already || got.Version != "01022030" {
		t.Errorf("Fetch of the same version = %+v, want it already there", got)
	}
	if got := fetch(t, set, zipFile, true); got.Already || got.Count != 8 {
		t.Errorf("Fetch --force = %+v, want 8 fetched", got)
	}

	hits, err := set.Search([]string{"LAMBDA"})
	if err != nil {
		t.Fatal(err)
	}
	wantHits := []icons.Hit{{Name: "aws/aws-lambda", Category: "Compute"}, {Name: "aws/res/aws-lambda-lambda-function", Category: "Compute"}}
	if !reflect.DeepEqual(hits, wantHits) {
		t.Errorf("Search lambda = %v, want %v", hits, wantHits)
	}
	if hits, serr := set.Search([]string{"lambda", "function"}); serr != nil || len(hits) != 1 {
		t.Errorf("Search lambda function = %v, %v, want the function only", hits, serr)
	}
	// A name as a spec writes it finds itself.
	if hits, serr := set.Search([]string{"aws/aws-lambda"}); serr != nil || len(hits) != 1 {
		t.Errorf("Search aws/aws-lambda = %v, %v, want aws/aws-lambda", hits, serr)
	}
	_, err = set.Search([]string{"lambda", "nothing"})
	if want := `floe: no icon name contains all of "lambda" "nothing"; try a shorter word`; told(err) != want {
		t.Errorf("Search without a hit = %s, want %s", told(err), want)
	}

	// A service in two categories keeps the first; an icon keeps its light variant, wherever its dark one
	// comes, and a dark variant alone is the icon.
	for name, want := range map[string]string{
		"aws-lambda": "Arch_Compute/48", "res/client": "Res_Client_48_Light", "res/server": "Res_Server_48_Light",
		"group/aws-cloud-logo": "AWS-Cloud-logo_32.svg", "res/night": "Res_Night_48_Dark",
	} {
		b, rerr := set.Lookup("aws/" + name)
		if rerr != nil || !strings.Contains(string(b), want) {
			t.Errorf("Lookup aws/%s = %s, %v, want the one of %s", name, b, rerr, want)
		}
	}
	// Nothing is left beside the icons.
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 {
		t.Errorf("the icons' root holds %v, %v; want aws alone", entries, err)
	}
}

func TestLookup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	set := icons.New(root)
	fetch(t, set, local(writeZip(t, "Icon-package.zip", packageFiles)), false)
	// A name its set does not hold, and one outside the set's directory, are not read.
	for _, name := range []string{"aws/amazon-ec2", "aws/../../etc/passwd", "aws/"} {
		if b, err := set.Lookup(name); !errors.Is(err, icons.ErrNoIcon) {
			t.Errorf("Lookup %s = %q, %v; want ErrNoIcon", name, b, err)
		}
	}
	// A name needs a known set in front.
	for _, name := range []string{"aws-lambda", "gcp/aws-lambda"} {
		if _, err := set.Lookup(name); !errors.Is(err, icons.ErrNoSet) {
			t.Errorf("Lookup %s = %v, want ErrNoSet", name, err)
		}
	}
	// An icon of the index that is gone is told at its file in the system's words.
	svg := filepath.Join(root, "aws", "aws-lambda.svg")
	if err := os.Remove(svg); err != nil {
		t.Fatal(err)
	}
	if _, err := set.Lookup("aws/aws-lambda"); told(err) != svg+": no such file or directory" {
		t.Errorf("Lookup of an icon gone = %s", told(err))
	}
	// The index is read once: a lookup after it is gone still finds the icon.
	if err := os.Remove(filepath.Join(root, "aws", "index.json")); err != nil {
		t.Fatal(err)
	}
	if b, err := set.Lookup("aws/res/client"); err != nil || len(b) == 0 {
		t.Errorf("Lookup after the index is gone = %q, %v; want the icon", b, err)
	}
}

func TestFetchFromURL(t *testing.T) {
	t.Parallel()

	zipBytes, err := os.ReadFile(writeZip(t, "Icon-package.zip", packageFiles))
	if err != nil {
		t.Fatal(err)
	}
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		if r.URL.Path != "/x/Icon-package_01022030.abc.zip" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(zipBytes) //nolint:errcheck // the test reads what arrives
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	set := icons.New(root)
	url := srv.URL + "/x/Icon-package_01022030.abc.zip"
	if got := fetch(t, set, icons.Source{Name: url, At: url}, false); got.Count != 8 {
		t.Errorf("Fetch from a URL = %+v", got)
	}
	missing := srv.URL + "/missing.zip"
	_, err = set.Fetch(context.Background(), "aws", icons.Source{Name: missing, At: missing}, true)
	if want := missing + ": cannot be downloaded (HTTP 404); check the network, or download it in a browser and pass its path"; told(err) != want {
		t.Errorf("Fetch of a missing URL = %s, want %s", told(err), want)
	}
	// The version in the URL is already there, so it is not downloaded again, nor its directory written: a
	// store made read-only after the fetch is no reason to stop.
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) }) //nolint:errcheck // for TempDir to remove it
	before := gets.Load()
	if got := fetch(t, set, icons.Source{Name: url, At: url}, false); !got.Already || gets.Load() != before {
		t.Errorf("Fetch of a URL of the same version = %+v after %d requests, want none", got, gets.Load()-before)
	}
}

// A store that cannot be written to is told before the download, as the store is written.
func TestFetchCannotWrite(t *testing.T) {
	t.Parallel()

	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) }) //nolint:errcheck // for TempDir to remove it
	root := filepath.Join(parent, "x")
	url := srv.URL + "/x/Icon-package_01022030.abc.zip"
	_, err := icons.New(root).Fetch(context.Background(), "aws", icons.Source{Name: url, At: url}, false)
	if os.Geteuid() == 0 {
		t.Skip("root writes to any directory")
	}
	want := root + ": cannot be written (permission denied): check that its directory exists and can be written to, " +
		"and that its name is free and not too long"
	if told(err) != want || gets.Load() != 0 {
		t.Errorf("Fetch into a directory that cannot be written = %s after %d requests, want %s before any", told(err), gets.Load(), want)
	}
}

// badZips are a zip whose directory claims one record more than it holds (truncated), and one whose stored
// entry does not match its CRC (checksum).
func badZips(t *testing.T) (truncated, checksum string) {
	t.Helper()
	b, err := os.ReadFile(writeZip(t, "Icon-package.zip", packageFiles[:2]))
	if err != nil {
		t.Fatal(err)
	}
	// The end of central directory record, without a comment, is the last 22 bytes; its counts at 8 and 10.
	end := len(b) - 22
	binary.LittleEndian.PutUint16(b[end+8:], 3)
	binary.LittleEndian.PutUint16(b[end+10:], 3)
	truncated = filepath.Join(t.TempDir(), "cut.zip")
	if err = os.WriteFile(truncated, b, 0o600); err != nil { //nolint:gosec // G703: the test's own temporary file
		t.Fatal(err)
	}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: packageFiles[0], Method: zip.Store}) //nolint:exhaustruct // a test's entry
	if err == nil {
		_, err = w.Write([]byte("<svg/>"))
	}
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	stored := zb.Bytes()
	// The stored data follows the 30-byte local header and the name: a changed byte breaks its CRC.
	stored[30+len(packageFiles[0])] ^= 0xff
	checksum = filepath.Join(t.TempDir(), "crc.zip")
	if err = os.WriteFile(checksum, stored, 0o600); err != nil {
		t.Fatal(err)
	}
	return truncated, checksum
}

func TestFetchRefusals(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	set := icons.New(root)
	dir := filepath.Join(root, "aws")
	notZip := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(notZip, []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := writeZip(t, "empty.zip", []string{"README.txt"})
	missing := filepath.Join(t.TempDir(), "none.zip")
	folder := t.TempDir()
	truncated, checksum := badZips(t)
	for _, c := range []struct {
		from icons.Source
		want string
	}{
		{local(notZip), "page.html: is not a zip file; pass the zip of the package on " + page},
		{local(empty), "empty.zip: holds no AWS icons; pass the zip of the package on " + page},
		{local(missing), "none.zip: no such file or directory"},
		{icons.Source{Name: "dir", At: folder}, "dir: is a directory"},
		{local(checksum), "crc.zip: cannot be read as a zip (zip: checksum error); download it again"},
		{local(truncated), "cut.zip: cannot be read as a zip (unexpected EOF); download it again"},
	} {
		if _, err := set.Fetch(context.Background(), "aws", c.from, false); told(err) != c.want {
			t.Errorf("Fetch %s = %s, want %s", c.from.Name, told(err), c.want)
		}
	}
	// Refusals leave nothing behind.
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Errorf("after refusals the icons' root holds %v, %v", entries, err)
	}

	// A broken index is refused by a search and a fetch, at its place, and replaced by a fetch --force.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(dir, "index.json")
	if err := os.WriteFile(index, []byte("{\n  \"version\": 1}"), 0o600); err != nil {
		t.Fatal(err)
	}
	zipFile := local(writeZip(t, "Icon-package.zip", packageFiles))
	want := index + `:2:14: cannot be read (json: cannot unmarshal number into Go struct field index.version of type string); ` +
		`run "floe icons fetch aws --force" to fetch the icons again`
	_, serr := set.Search([]string{"lambda"})
	_, ferr := set.Fetch(context.Background(), "aws", zipFile, false)
	for _, err := range []error{serr, ferr} {
		if told(err) != want {
			t.Errorf("a broken index = %s, want %s", told(err), want)
		}
	}
	if got := fetch(t, set, zipFile, true); got.Count != 8 {
		t.Errorf("Fetch --force over a broken index = %+v", got)
	}

	// A set floe cannot fetch is refused by name, and is not a name without a set.
	_, err := set.Fetch(context.Background(), "gcp", icons.Source{Name: "", At: ""}, false)
	if want := `floe: unknown icon set "gcp"; use aws`; told(err) != want || errors.Is(err, icons.ErrNoSet) {
		t.Errorf("Fetch gcp = %s, want %s", told(err), want)
	}
}

// An index that is no JSON is told at the place encoding/json names, and one that cannot be read in the
// system's words, as fetching again would not mend it.
func TestBrokenIndex(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		index, want string
	}{
		{"{", `:1:1: cannot be read (unexpected end of JSON input); run "floe icons fetch aws --force" to fetch the icons again`},
		{`{"icons": "日"}`, `:1:13: cannot be read (json: cannot unmarshal string into Go struct field index.icons of type map[string]icons.entry); ` +
			`run "floe icons fetch aws --force" to fetch the icons again`},
	} {
		dir := filepath.Join(t.TempDir(), "aws")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		index := filepath.Join(dir, "index.json")
		if err := os.WriteFile(index, []byte(c.index), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := icons.New(filepath.Dir(dir)).Search([]string{"x"}); told(err) != index+c.want {
			t.Errorf("index %s = %s, want %s", c.index, told(err), index+c.want)
		}
	}
	// A root that is a file: what is under it is no directory.
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := icons.New(file).Search([]string{"x"}); told(err) != file+"/aws/index.json: not a directory" {
		t.Errorf("a root that is a file = %s", told(err))
	}
	_, err := icons.New(file).Fetch(context.Background(), "aws", local(writeZip(t, "Icon-package.zip", packageFiles)), true)
	if want := file + ": cannot be written (not a directory): check"; !strings.HasPrefix(told(err), want) {
		t.Errorf("a fetch --force into a root that is a file = %s, want %s…", told(err), want)
	}
}

// A set not fetched is told once, for goroutines looking up in one store at once too.
func TestNotFetchedOnce(t *testing.T) {
	t.Parallel()

	s := icons.New(t.TempDir())
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { _, errs[i] = s.Lookup("aws/aws-lambda") })
	}
	wg.Wait()
	r := usererr.Tell(i18n.EnglishCatalog(), usererr.Collect(errs...))
	if len(r.Mistakes) != 1 || !strings.Contains(r.Mistakes[0].Text, "not in") {
		t.Fatalf("a set not fetched looked up at once = %q, want it told once", r.Render(""))
	}
}
