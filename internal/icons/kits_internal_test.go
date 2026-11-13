package icons

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"floe/internal/usererr"
)

// wantMistake fails unless err is the mistake key, its arguments holding args.
func wantMistake(t *testing.T, what string, err error, key string, args ...any) {
	t.Helper()
	var e *usererr.Error
	if !errors.As(err, &e) || e.Key != key {
		t.Errorf("%s = %v, want %s", what, err, key)
		return
	}
	for _, a := range args {
		if !slices.Contains(e.Args, a) {
			t.Errorf("%s: %s holds %q, not %q", what, key, e.Args, a)
		}
	}
}

// writeZipOf writes a zip of the entries, each holding its content, to a new file, and returns it as a source.
func writeZipOf(t *testing.T, name string, entries map[string]string) Source {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, path := range slices.Sorted(func(yield func(string) bool) {
		for p := range entries {
			if !yield(p) {
				return
			}
		}
	}) {
		w, err := zw.Create(path)
		if err == nil {
			_, err = w.Write([]byte(entries[path]))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return Source{Name: name, At: file}
}

var (
	gcpCoreZip = map[string]string{
		"Unique Icons/Cloud Run/SVG/CloudRun-512-color-rgb.svg": "<svg>run</svg>",
		"Unique Icons/Cloud Run/PNG/CloudRun-512-color.png":     "png",
		"Unique Icons/GKE/SVG/GKE-512-color.svg":                "<svg>gke</svg>",
	}
	gcpCategoryZip = map[string]string{
		"Category Icons/Networking/SVG/Networking-512-color-rgb.svg":  "<svg>net</svg>",
		"Category Icons/AI _ Machine Learning/SVG/AIML-512-color.svg": "<svg>ai</svg>",
		"Category Icons/.DS_Store":                                    "x",
	}
	azureZip = map[string]string{
		"Azure_Public_Service_Icons/Icons/compute/10021-icon-service-Virtual-Machine.svg": "<svg>vm</svg>",
		"Azure_Public_Service_Icons/Icons/new icons/030777508 -icon-service-Groups.svg":   "<svg>g</svg>",
	}
)

// Every SVG of the packages of 2026-11 is read, by the pack its zip is; nothing else of them is.
func TestRealPaths(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		k     *kit
		file  string
		pack  int
		count int // icons, a name kept once
		names []string
	}{
		{&gcp, "gcp-core.txt", 0, 19, []string{"cloud-run", "gke", "compute-engine", "cloud-storage"}},
		{&gcp, "gcp-category.txt", 1, 26, []string{"category/networking", "category/ai-machine-learning", "category/web-mobile"}},
		{&azure, "azure.txt", 0, 643, []string{"virtual-machine", "arc-postgresql", "service-group-relationships", "cloud-services-classic"}},
	} {
		f, err := os.Open(filepath.Join("testdata", "paths", c.file))
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		lines := bufio.NewScanner(f)
		for lines.Scan() {
			path := lines.Text()
			icon, p, ok := c.k.iconOf(path)
			if ok != strings.HasSuffix(path, ".svg") {
				t.Errorf("%s: %s read %v", c.file, path, ok)
			}
			if ok && p != c.pack {
				t.Errorf("%s: %s read as of pack %d, want %d", c.file, path, p, c.pack)
			}
			if ok {
				names[icon.name] = true
			}
		}
		_ = f.Close() //nolint:errcheck // read only
		if len(names) != c.count {
			t.Errorf("%s: %d icons, want %d", c.file, len(names), c.count)
		}
		for _, n := range c.names {
			if !names[n] {
				t.Errorf("%s: no icon %s", c.file, n)
			}
		}
	}
}

// A name is lower case, each run of what is not a letter, a digit or a dot a dash, none at its ends; one of none of
// them names nothing, and its path is not read.
func TestSlug(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"Arc-PostgreSQL ":                                "arc-postgresql",
		"Cloud-Services-(Classic)":                       "cloud-services-classic",
		"AI _ Machine Learning":                          "ai-machine-learning",
		"AWS-Elemental-Appliances-&-Software":            "aws-elemental-appliances-software",
		"Amazon-EC2_AWS-Microservice-Extractor-for-.NET": "amazon-ec2-aws-microservice-extractor-for-.net",
		"(&)": "",
		"___": "",
	} {
		if got := slug(raw); got != want {
			t.Errorf("slug(%q) = %q, want %q", raw, got, want)
		}
	}
	for _, path := range []string{
		"Category Icons/(&)/SVG/x.svg",
		"Unique Icons/___/SVG/x.svg",
		"Azure_Public_Service_Icons/Icons/x/00001-icon-service-___.svg",
		"Architecture-Group-Icons_01022030/(&)_32.svg",
	} {
		for _, k := range kits {
			if icon, _, ok := k.iconOf(path); ok {
				t.Errorf("%s: %s read as %q", k.name, path, icon.name)
			}
		}
	}
}

// A set takes no more zips than it has packages, told before anything else is done.
func TestCheckSources(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		set  string
		n    int
		fail bool
	}{
		{"gcp", 0, false}, {"gcp", 2, false}, {"gcp", 3, true},
		{"aws", 1, false}, {"aws", 2, true}, {"azure", 2, true}, {"nope", 5, false},
	} {
		err := CheckSources(c.set, c.n)
		if !c.fail {
			if err != nil {
				t.Errorf("CheckSources(%s, %d) = %v", c.set, c.n, err)
			}
			continue
		}
		k := kitOf(c.set)
		wantMistake(t, "CheckSources("+c.set+")", err, "icons_too_many_zips", k.title, k.zips(), k.usage())
	}
	// Fetch refuses them too, writing nothing.
	root := t.TempDir()
	core := writeZipOf(t, "core.zip", gcpCoreZip)
	_, err := New(root, false, "").Fetch(context.Background(), "aws", []Source{core, core}, false)
	wantMistake(t, "Fetch aws of 2", err, "icons_too_many_zips", "floe icons fetch aws <path-or-url>")
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("a refused fetch left %v", entries)
	}
}

// Google Cloud needs both its zips, in either order; a zip of neither is told for itself.
func TestFetchPacks(t *testing.T) {
	t.Parallel()

	core := writeZipOf(t, "core.zip", gcpCoreZip)
	category := writeZipOf(t, "category.zip", gcpCategoryZip)
	other := writeZipOf(t, "other.zip", azureZip)
	set := New(t.TempDir(), false, "")
	got, err := set.Fetch(context.Background(), "gcp", []Source{category, core}, false)
	if err != nil || got.Count != 4 {
		t.Fatalf("Fetch gcp of both = %+v, %v; want 4 icons", got, err)
	}
	for _, name := range []string{"gcp/cloud-run", "gcp/gke", "gcp/category/networking", "gcp/category/ai-machine-learning"} {
		if _, err := set.Lookup(name); err != nil {
			t.Errorf("Lookup %s = %v", name, err)
		}
	}
	for _, c := range []struct {
		what string
		from []Source
		key  string
		args []any
	}{
		{"one zip", []Source{core}, "icons_pack_missing", []any{"Google Cloud", "the product category icons zip", gcp.zips(), gcp.page, gcp.usage()}},
		{"the same zip twice", []Source{core, core}, "icons_pack_missing", []any{"the product category icons zip"}},
		{"a zip of another set", []Source{core, other}, "icons_none_in_zip", []any{"Google Cloud", gcp.zips(), gcp.page, gcp.usage()}},
	} {
		_, err := New(t.TempDir(), false, "").Fetch(context.Background(), "gcp", c.from, true)
		wantMistake(t, c.what, err, c.key, c.args...)
	}
	_, err = New(t.TempDir(), false, "").Fetch(context.Background(), "azure", []Source{core}, true)
	wantMistake(t, "a Google Cloud zip for Azure", err, "icons_none_in_zip", "Azure", azure.zips(), azure.page, azure.usage())
	notZip := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(notZip, []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = New(t.TempDir(), false, "").Fetch(context.Background(), "gcp", []Source{{Name: "page.html", At: notZip}}, true)
	wantMistake(t, "no zip", err, "icons_not_zip", gcp.zips(), gcp.page, gcp.usage())
}

// Azure is fetched only from a zip given: without one, nothing is downloaded. Not fetched, it is told once in all
// that is drawn, with where to agree and download.
func TestFetchTerms(t *testing.T) { //nolint:paralleltest // changes azure's page
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { asked.Add(1) }))
	t.Cleanup(srv.Close)
	set(t, &azure.page, srv.URL+"/icons/")
	store := New(t.TempDir(), false, "")
	_, err := store.Fetch(context.Background(), "azure", nil, false)
	wantMistake(t, "Fetch azure without a zip", err, "icons_terms", "Azure", azure.page, azure.zips(), azure.usage())
	if asked.Load() != 0 {
		t.Errorf("a fetch without a zip asked the network %d times", asked.Load())
	}
	_, err = store.Lookup("azure/virtual-machine")
	wantMistake(t, "Lookup before a fetch", err, "icons_not_fetched_terms", "Azure", azure.page, azure.usage())
	var e *usererr.Error
	// One of the whole output stays unplaced.
	if errors.As(err, &e) && e.PlacedAt("page.yaml", 1, 1).File() != "" {
		t.Errorf("Lookup before a fetch = %v, not a mistake of the whole output", err)
	}
	if got, err := store.Fetch(context.Background(), "azure", []Source{writeZipOf(t, "azure.zip", azureZip)}, false); err != nil || got.Count != 2 {
		t.Errorf("Fetch azure of its zip = %+v, %v", got, err)
	}
}

// A version of icons whose package holds none is their hash: the same whatever order the zips come in, another when
// an icon's content or category changes.
func TestHashVersion(t *testing.T) {
	t.Parallel()

	core := writeZipOf(t, "core.zip", gcpCoreZip)
	category := writeZipOf(t, "category.zip", gcpCategoryZip)
	set := New(t.TempDir(), false, "")
	first, err := set.Fetch(context.Background(), "gcp", []Source{core, category}, false)
	if err != nil || len(first.Version) != 8 {
		t.Fatalf("Fetch gcp = %+v, %v", first, err)
	}
	if again, err := set.Fetch(context.Background(), "gcp", []Source{category, core}, false); err != nil || !again.Already {
		t.Errorf("Fetch of the zips in the other order = %+v, %v; want already", again, err)
	}
	changed := map[string]string{}
	for p, c := range gcpCoreZip {
		changed[p] = c
	}
	changed["Unique Icons/GKE/SVG/GKE-512-color.svg"] = "<svg>gke!</svg>"
	if got, err := set.Fetch(context.Background(), "gcp", []Source{writeZipOf(t, "c2.zip", changed), category}, false); err != nil || got.Already || got.Version == first.Version {
		t.Errorf("Fetch of an icon changed = %+v, %v; want another version", got, err)
	}

	azureSet := New(t.TempDir(), false, "")
	before, err := azureSet.Fetch(context.Background(), "azure", []Source{writeZipOf(t, "a.zip", azureZip)}, false)
	if err != nil {
		t.Fatal(err)
	}
	moved := map[string]string{}
	for p, c := range azureZip {
		moved[strings.Replace(p, "/compute/", "/other/", 1)] = c
	}
	if got, err := azureSet.Fetch(context.Background(), "azure", []Source{writeZipOf(t, "b.zip", moved)}, false); err != nil || got.Version == before.Version {
		t.Errorf("Fetch of an icon in another category = %+v, %v; want another version than %s", got, err, before.Version)
	}
}

// The limits count the icons of all the zips of a fetch together.
func TestLimitsInAll(t *testing.T) { //nolint:paralleltest // changes the limits
	core := writeZipOf(t, "core.zip", gcpCoreZip)             // 2 icons, 27 bytes
	category := writeZipOf(t, "category.zip", gcpCategoryZip) // 2 icons, 27 bytes
	set(t, &maxIcons, 3)
	_, err := New(t.TempDir(), false, "").Fetch(context.Background(), "gcp", []Source{core, category}, true)
	wantMistake(t, "4 icons over 3", err, "icons_too_many", 3)
	if e := (*usererr.Error)(nil); errors.As(err, &e) && e.File() != "category.zip" {
		t.Errorf("icons_too_many at %q, want the zip that went over", e.File())
	}
	set(t, &maxIcons, 10)
	set(t, &maxPackage, 40)
	_, err = New(t.TempDir(), false, "").Fetch(context.Background(), "gcp", []Source{core, category}, true)
	wantMistake(t, "54 bytes over 40", err, "icons_written_too_large")
}

// The command that fetches again is the set's: a set behind terms takes its zip.
func TestAgain(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "index.json")
	for _, c := range []struct {
		k    *kit
		want string
	}{
		{&aws, "floe icons fetch aws --force"},
		{&gcp, "floe icons fetch gcp --force"},
		{&azure, "floe icons fetch azure <path-or-url> --force"},
	} {
		_, err := parseIndex(file, []byte("{"), c.k)
		wantMistake(t, c.k.name+" broken", err, "icons_broken", c.want)
		_, err = parseIndex(file, []byte("{}"), c.k)
		wantMistake(t, c.k.name+" without a version", err, "icons_index_no_version", c.want)
		wantMistake(t, c.k.name+" not a file", (&Store{}).cannotRead(file, usererr.ErrNotFile, c.k), "icons_not_file", c.want)
	}
}

// Google Cloud's page links both zips and the legacy one: the two are taken; one missing is told for itself.
func TestPackageURLs(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		page string
		want []string // "" for the mistake
	}{
		{
			`<a href="https://s.example/misc/category-icons.zip">` + `<a href="/misc/google-cloud-legacy-icons.zip">` +
				`<a href="https://s.example/misc/core-products-icons.zip">`,
			[]string{"https://s.example/misc/core-products-icons.zip", "https://s.example/misc/category-icons.zip"},
		},
		{
			`<a href="https://s.example/1/core-products-icons.zip"><a href="https://s.example/1/category-icons.zip">` +
				`<a href="https://s.example/2/category-icons.zip"><a href="https://s.example/2/core-products-icons.zip">`,
			[]string{"https://s.example/1/core-products-icons.zip", "https://s.example/1/category-icons.zip"},
		},
		{`<a href="https://s.example/misc/core-products-icons.zip"><a href="/misc/google-cloud-legacy-icons.zip">`, nil},
	} {
		var agent atomic.Value
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agent.Store(r.UserAgent())
			_, _ = w.Write([]byte(c.page)) //nolint:errcheck // the test reads what arrives
		}))
		k := gcp
		k.page = srv.URL + "/icons"
		got, err := k.packageURLs(context.Background())
		srv.Close()
		if agent.Load() != "floe" {
			t.Errorf("the page was asked as %q, want floe (cloud.google.com answers Go's own with no page)", agent.Load())
		}
		if c.want == nil {
			wantMistake(t, "a page without the category zip", err, "icons_no_package_link", "the product category icons zip", gcp.zips(), gcp.usage())
			continue
		}
		var ats []string
		for _, s := range got {
			ats = append(ats, s.At)
		}
		if err != nil || !slices.Equal(ats, c.want) {
			t.Errorf("packageURLs = %v, %v; want %v", ats, err, c.want)
		}
	}
	// A page or zip that cannot be downloaded asks for the set's zips.
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	k := gcp
	k.page = srv.URL + "/icons"
	_, err := k.packageURLs(context.Background())
	wantMistake(t, "a page not there", err, "icons_page_download", "HTTP 404", gcp.zips(), gcp.usage())
	_, err = New(t.TempDir(), false, "").Fetch(context.Background(), "gcp", []Source{{Name: srv.URL + "/a.zip", At: srv.URL + "/a.zip"}}, true)
	wantMistake(t, "a zip not there", err, "icons_download", "HTTP 404", gcp.zips(), gcp.usage())
}

// A gcp or azure path of .. is no name, as an AWS one is not.
func TestNewIconOfStaysInside(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"Unique Icons/../SVG/x.svg",
		"Category Icons/../../SVG/x.svg",
		"Azure_Public_Service_Icons/Icons/x/00001-icon-service-../../x.svg",
	} {
		for _, k := range []*kit{&gcp, &azure} {
			if icon, _, ok := k.iconOf(path); ok && !filepath.IsLocal(icon.name+".svg") {
				t.Errorf("%s: %s keeps %s", k.name, path, icon.name)
			}
		}
	}
}
