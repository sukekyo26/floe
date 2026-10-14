package cli_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const spec = "template: report\nlang: ja\ntitle: Spec\nsections:\n  - parts:\n      - text: Do **it**.\n"

// floe render writes each page beside its data file, or in --out-dir, only when it changes; --check
// tells a page missing or not what its data file draws.
func TestRender(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.write(t, "work/spec.yaml", spec)
	html := filepath.Join(e.tmp, "work", "spec.html")
	want(t, "render", e.run(t, "", "render", "work/spec.yaml"), 0, "work/spec.html\n", "")
	page, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`<html lang="ja">`, `<meta http-equiv="Content-Security-Policy" content="default-src 'none';`, "<p>Do <strong>it</strong>.</p>"} {
		if !strings.Contains(string(page), w) {
			t.Errorf("the page lacks %s:\n%s", w, page)
		}
	}
	want(t, "render again", e.run(t, "work", "render", "spec.yaml"), 0, "", "")
	e.write(t, "work/other.yaml", spec)
	want(t, "render a file named twice", e.run(t, "work", "render", "spec.yaml", "./other.yaml", "other.yaml"), 0, "other.html\n", "")
	if err := os.Symlink(filepath.Join(e.tmp, "work"), filepath.Join(e.tmp, "link")); err != nil {
		t.Fatal(err)
	}
	want(t, "render a file named through a symlink too", e.run(t, "", "render", "work/spec.yaml", "link/spec.yaml"), 0, "", "")
	// A symlink of another name is another page.
	if err := os.Symlink("spec.yaml", filepath.Join(e.tmp, "work", "latest.yaml")); err != nil {
		t.Fatal(err)
	}
	want(t, "render a symlink of another name", e.run(t, "work", "render", "spec.yaml", "latest.yaml"), 0, "latest.html\n", "")
	// A name without a letter a slug takes titles the page by itself.
	e.write(t, "work/仕様.yaml", strings.Replace(spec, "title: Spec\n", "", 1))
	want(t, "render a name the slug drops", e.run(t, "work", "render", "仕様.yaml"), 0, "仕様.html\n", "")
	if b, err := os.ReadFile(filepath.Join(e.tmp, "work", "仕様.html")); err != nil || !bytes.Contains(b, []byte("<title>仕様</title>")) {
		t.Errorf("the page of 仕様.yaml = %v\n%s", err, b)
	}
	want(t, "check", e.run(t, "work", "render", "--check", "spec.yaml"), 0, "", "")

	e.write(t, "work/spec.yaml", strings.Replace(spec, "Do", "Did", 1))
	want(t, "check a page not written again", e.run(t, "work", "render", "--check", "spec.yaml"), 1, "",
		"spec.html is not what spec.yaml draws: write it again with floe render")
	if b, err := os.ReadFile(html); err != nil || !bytes.Equal(b, page) {
		t.Errorf("--check wrote the page: %v", err)
	}
	want(t, "check a missing page", e.run(t, "work", "render", "--check", "spec.yaml", "--out-dir", "out"), 1, "",
		"out/spec.html is missing: write it from spec.yaml with floe render")
	want(t, "render into --out-dir", e.run(t, "work", "render", "spec.yaml", "--out-dir", "out/pages"), 0, "out/pages/spec.html\n", "")
	want(t, "check --out-dir", e.run(t, "work", "render", "--check", "spec.yaml", "--out-dir", "out/pages"), 0, "", "")
}

// Nothing is written while a data file has mistakes, or two data files would write the same page.
func TestRenderRefusals(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.write(t, "spec.yaml", spec)
	e.write(t, "sub/spec.yaml", spec)
	e.write(t, "sub/Spec.yaml", spec)
	e.write(t, "bad.yaml", "template: report\nleed: x\nlang: en\n")
	want(t, "render with a mistake", e.run(t, "", "render", "spec.yaml", "bad.yaml"), 1, "",
		"bad.yaml:2:1: leed is not an item of the template report")
	want(t, "render onto one page", e.run(t, "", "render", "spec.yaml", "sub/spec.yaml", "--out-dir", "out"), 1, "",
		"spec.yaml and sub/spec.yaml would both write out/spec.html; rename one of them")
	// On macOS, Spec.html and spec.html are one file.
	want(t, "render onto one page but for case", e.run(t, "", "render", "spec.yaml", "sub/Spec.yaml", "--out-dir", "out"), 1, "",
		"spec.yaml and sub/Spec.yaml would both write out/Spec.html")
	for _, f := range []string{"spec.html", "out"} {
		if _, err := os.Stat(filepath.Join(e.tmp, f)); !os.IsNotExist(err) {
			t.Errorf("%s was written: %v", f, err)
		}
	}
	// A file with mistakes still takes its page, and two files reach one page through a symlinked directory.
	e.write(t, "sub/bad.yaml", spec)
	want(t, "render a file with mistakes onto one page", e.run(t, "", "render", "bad.yaml", "sub/bad.yaml", "--out-dir", "out"), 1, "",
		"bad.yaml:2:1: leed is not an item of the template report (a typo?); use eyebrow, lang, lead, sections, template, theme, title\n"+
			"bad.yaml and sub/bad.yaml would both write out/bad.html")
	e.write(t, "sub/spec.yml", spec)
	if err := os.Symlink("sub", filepath.Join(e.tmp, "alias")); err != nil {
		t.Fatal(err)
	}
	want(t, "render onto one page through a symlink", e.run(t, "", "render", "sub/spec.yaml", "alias/spec.yml"), 1, "",
		"sub/spec.yaml and alias/spec.yml would both write alias/spec.html")
	want(t, "render without a file", e.run(t, "", "render"), 2, "", "floe: requires at least 1 arg(s)")
}

// The page floe render writes is the download of the page floe publish stores: drawn the same each time,
// and titled the same, by the slug, without a title:.
func TestRenderIsTheDownload(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.write(t, "My_Spec.yaml", strings.Replace(spec, "title: Spec\n", "", 1))
	want(t, "render", e.run(t, "", "render", "My_Spec.yaml"), 0, "My_Spec.html\n", "")
	want(t, "publish", e.run(t, "", "publish", "My_Spec.yaml"), 0, e.base()+"/a/my-spec/\n", "")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, e.base()+"/api/artifacts/my-spec/download", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only
	download, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if page, err := os.ReadFile(filepath.Join(e.tmp, "My_Spec.html")); err != nil || !bytes.Equal(page, download) ||
		!bytes.Contains(page, []byte("<title>my-spec</title>")) {
		t.Errorf("the page = %v\n%s\nthe download =\n%s", err, page, download)
	}
}
