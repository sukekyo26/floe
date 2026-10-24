package icons_test

import (
	"archive/zip"
	"context"
	"encoding/json"
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

	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
)

// packageFiles stand in for the Icon package: what is kept, a service in two categories, sizes and
// folders that are not kept, a resource with only a dark variant, and macOS's resource forks.
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

func fetch(t *testing.T, set *icons.Store, source string, force bool) icons.Fetched {
	t.Helper()
	got, err := set.Fetch(context.Background(), "aws", source, force)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestFetchAndSearch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	set := icons.New(root)
	dir := filepath.Join(root, "aws")
	// Before a fetch, a search and a lookup say how to fetch.
	_, err := set.Search([]string{"lambda"})
	if want := "no icons are in " + root + `; fetch a set with "floe icons fetch <set>" (aws) and try again`; err == nil || err.Error() != want {
		t.Errorf("Search before a fetch = %v, want %s", err, want)
	}
	_, _, err = set.Lookup("aws/aws-lambda")
	want := "AWS icons are not in " + dir + `; run "floe icons fetch aws" (downloads about 14MB from aws.amazon.com) and try again`
	if err == nil || err.Error() != want {
		t.Errorf("Lookup before a fetch = %v, want %s", err, want)
	}

	zipFile := writeZip(t, "Icon-package.zip", packageFiles)
	got := fetch(t, set, zipFile, false)
	if want := (icons.Fetched{Title: "AWS", Dir: dir, Version: "01022030", Count: 7, Already: false}); got != want {
		t.Errorf("Fetch = %+v, want %+v", got, want)
	}
	// The same version again is kept, unless forced.
	if got := fetch(t, set, zipFile, false); !got.Already || got.Version != "01022030" {
		t.Errorf("Fetch of the same version = %+v, want it already there", got)
	}
	if got := fetch(t, set, zipFile, true); got.Already || got.Count != 7 {
		t.Errorf("Fetch --force = %+v, want 7 fetched", got)
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
	if want := `no icon name contains all of "lambda" "nothing"; try a shorter word`; err == nil || err.Error() != want {
		t.Errorf("Search without a hit = %v, want %s", err, want)
	}

	// A service in two categories keeps the first; dark variants are kept, and a dark variant alone
	// is the icon's usual one.
	b, err := os.ReadFile(filepath.Join(dir, "aws-lambda.svg"))
	if err != nil || !strings.Contains(string(b), "Arch_Compute/48") {
		t.Errorf("aws-lambda.svg = %s, %v, want the one of Compute", b, err)
	}
	var idx struct {
		Icons map[string]struct {
			Dark bool `json:"dark"`
		} `json:"icons"`
	}
	b, err = os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil || json.Unmarshal(b, &idx) != nil {
		t.Fatalf("index.json = %s, %v", b, err)
	}
	dark := map[string]bool{}
	for _, name := range []string{"res/client", "group/aws-cloud-logo", "group/region", "res/night"} {
		dark[name] = idx.Icons[name].Dark
	}
	if want := map[string]bool{"res/client": true, "group/aws-cloud-logo": true, "group/region": false, "res/night": false}; !reflect.DeepEqual(dark, want) {
		t.Errorf("dark variants = %v, want %v", dark, want)
	}
	icon, _, err := set.Lookup("aws/res/night")
	if err != nil || icon == nil || icon.Dark != "" || !strings.HasPrefix(icon.Light, "data:image/svg+xml;base64,") {
		t.Errorf("Lookup of a dark variant alone = %+v, %v", icon, err)
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
	fetch(t, set, writeZip(t, "Icon-package.zip", packageFiles), false)
	icon, names, err := set.Lookup("aws/res/client")
	if err != nil || icon == nil || names != nil || icon.Light == "" || icon.Dark == "" || icon.Light == icon.Dark {
		t.Errorf("Lookup res/client = %+v, %v, %v; want both variants", icon, names, err)
	}
	// An unknown name gets the names sharing the most words, shortest first.
	icon, names, err = set.Lookup("aws/Lambda_Function")
	if want := []string{"aws/res/aws-lambda-lambda-function", "aws/aws-lambda"}; err != nil || icon != nil || !reflect.DeepEqual(names, want) {
		t.Errorf("Lookup lambda-function = %+v, %v, %v; want %v", icon, names, err, want)
	}
	// A misspelt word still finds the name.
	if _, names, err := set.Lookup("aws/lamda"); err != nil || len(names) == 0 || names[0] != "aws/aws-lambda" {
		t.Errorf("Lookup aws/lamda = %v, %v; want aws/aws-lambda first", names, err)
	}
	// A misspelt common word finds nothing by itself, so the other words decide.
	if _, names, err := set.Lookup("aws/amazn-lambda"); err != nil || len(names) != 2 || names[0] != "aws/aws-lambda" {
		t.Errorf("Lookup aws/amazn-lambda = %v, %v; want the two lambdas, aws/aws-lambda first", names, err)
	}
	if icon, names, err := set.Lookup("aws/amazon-ec2"); err != nil || icon != nil || len(names) != 0 {
		t.Errorf("Lookup amazon-ec2 = %+v, %v, %v; want no close names", icon, names, err)
	}
	// The index is read once: a lookup after it is gone still finds the icon.
	if err := os.Remove(filepath.Join(root, "aws", "index.json")); err != nil {
		t.Fatal(err)
	}
	if icon, _, err := set.Lookup("aws/aws-lambda"); err != nil || icon == nil {
		t.Errorf("Lookup after the index is gone = %+v, %v; want the icon", icon, err)
	}
	// A name needs a known set in front.
	for _, name := range []string{"aws-lambda", "gcp/aws-lambda"} {
		if _, _, err := set.Lookup(name); !errors.Is(err, icons.ErrNoSet) {
			t.Errorf("Lookup %s = %v, want ErrNoSet", name, err)
		}
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
	if got := fetch(t, set, url, false); got.Count != 7 {
		t.Errorf("Fetch from a URL = %+v", got)
	}
	// The version in the URL is already there, so it is not downloaded again.
	if got := fetch(t, set, url, false); !got.Already || gets.Load() != 1 {
		t.Errorf("Fetch of a URL of the same version = %+v after %d downloads, want 1", got, gets.Load())
	}
	_, err = set.Fetch(context.Background(), "aws", srv.URL+"/missing.zip", false)
	if want := "cannot download " + srv.URL + "/missing.zip (HTTP 404)"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("Fetch of a missing URL = %v, want %s…", err, want)
	}
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
	for source, want := range map[string]string{
		notZip:  notZip + " is not a zip file; pass the zip of the package on " + page,
		empty:   "no AWS icons in " + empty + "; pass the zip of the package on " + page,
		missing: "cannot read " + missing + " (no such file or directory); check the path and that the file exists",
	} {
		if _, err := set.Fetch(context.Background(), "aws", source, false); err == nil || err.Error() != want {
			t.Errorf("Fetch %s = %v, want %s", source, err, want)
		}
	}
	// Refusals leave nothing behind.
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Errorf("after refusals the icons' root holds %v, %v", entries, err)
	}

	// A broken index is refused by a search, and replaced by a fetch.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := set.Search([]string{"lambda"}); err == nil || !strings.Contains(err.Error(), `run "floe icons fetch aws --force"`) {
		t.Errorf("Search with a broken index = %v", err)
	}
	if got := fetch(t, set, writeZip(t, "Icon-package.zip", packageFiles), false); got.Count != 7 {
		t.Errorf("Fetch over a broken index = %+v", got)
	}

	// A set floe cannot fetch is refused by name, and is not a name without a set.
	_, err := set.Fetch(context.Background(), "gcp", "", false)
	if want := `unknown icon set "gcp"; use aws`; err == nil || err.Error() != want || errors.Is(err, icons.ErrNoSet) {
		t.Errorf("Fetch gcp = %v, want %s", err, want)
	}

	// Without a home to put them under, the icons are refused when used.
	nowhere := icons.New("")
	_, serr := nowhere.Search([]string{"lambda"})
	_, ferr := nowhere.Fetch(context.Background(), "aws", empty, false)
	for _, err := range []error{serr, ferr} {
		if want := `there is no directory for icons; set FLOE_ICONS_DIR, then fetch a set with "floe icons fetch <set>"`; err == nil || err.Error() != want {
			t.Errorf("icons without a root = %v, want %s", err, want)
		}
	}
}

// The mistake of a set not fetched is made once in a store, for goroutines looking up at once too.
func TestNotFetchedOnce(t *testing.T) {
	t.Parallel()

	s := icons.New(t.TempDir())
	got := make([]*usererr.Error, 8)
	var wg sync.WaitGroup
	for i := range got {
		wg.Go(func() {
			_, _, err := s.Lookup("aws/aws-lambda")
			errors.As(err, &got[i])
		})
	}
	wg.Wait()
	for _, e := range got {
		if e == nil || e != got[0] {
			t.Fatalf("the mistakes of a set not fetched looked up at once = %v, want one", got)
		}
	}
}
