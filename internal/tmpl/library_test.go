package tmpl_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// own writes a copy of the built-in report as the template name under dir, with page.html marked so
// that a page tells which report drew it.
func own(t *testing.T, dir, name string) string {
	t.Helper()

	at := filepath.Join(dir, name)
	if err := os.CopyFS(at, os.DirFS("builtin/report")); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(at, "page.html")
	src, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, append(src, "<i>own</i>"...), 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	return at
}

func draw(l *tmpl.Library, src string) (string, error) {
	d, err := l.Read("data.yaml", []byte(src), "", "")
	if err != nil {
		return "", err
	}
	b, err := d.Render("t", "en")
	return string(b), err
}

// A template of one's own takes the place of the built-in one of its name, a symlinked one included,
// and the names list both kinds once; files and names starting with a dot are not templates.
func TestOwnTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	own(t, dir, "report")
	elsewhere := own(t, t.TempDir(), "memo")
	if err := os.Symlink(elsewhere, filepath.Join(dir, "memo")); err != nil {
		t.Fatal(err)
	}
	own(t, dir, ".installing")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l := tmpl.NewLibrary(dir, "")
	if got, err := l.Names(); err != nil || !slices.Equal(got, []string{"memo", "report"}) {
		t.Errorf("Names = %v, %v", got, err)
	}
	for _, name := range []string{"report", "memo"} {
		if page, err := draw(l, "template: "+name+"\n"); err != nil || !strings.Contains(page, "<i>own</i>") {
			t.Errorf("a page of %s = %v, drawn by the built-in one", name, err)
		}
	}
	if page, err := draw(tmpl.NewLibrary("", ""), "template: report\n"); err != nil || strings.Contains(page, "<i>own</i>") {
		t.Errorf("a page of the built-in report = %v", err)
	}
	// A file where the directory of templates would be holds none.
	if _, err := draw(tmpl.NewLibrary(filepath.Join(dir, "notes.txt"), ""), "template: report\n"); err != nil {
		t.Errorf("a page of report under a file in the way = %v", err)
	}
	for _, name := range []string{"../report", ".installing", "notes.txt", "a/b", "Report", "memo/"} {
		if _, err := draw(l, "template: "+name+"\n"); err == nil || !strings.Contains(err.Error(), "unknown template") {
			t.Errorf("template %s = %v, want unknown", name, err)
		}
	}
}

// A broken template of one's own stops, and does not give way to the built-in one: a file it lacks,
// a symlink that leads nowhere, and a file that leads out of it.
func TestBrokenOwnTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Remove(filepath.Join(own(t, dir, "report"), "page.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, ""), "template: report\n"); err == nil || !strings.Contains(err.Error(), "page.html") {
		t.Errorf("a template without page.html = %v", err)
	}

	dangling := t.TempDir()
	if err := os.Symlink(filepath.Join(dangling, "gone"), filepath.Join(dangling, "report")); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dangling, ""), "template: report\n"); err == nil || !strings.Contains(err.Error(), "your own template report (") {
		t.Errorf("a symlink that leads nowhere = %v", err)
	}

	leaky := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.css")
	if err := os.WriteFile(outside, []byte("body{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	theme := filepath.Join(own(t, leaky, "report"), "themes", "default.css")
	if err := os.Remove(theme); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, theme); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(leaky, ""), "template: report\n"); err == nil || !strings.Contains(err.Error(), "themes/default.css") {
		t.Errorf("a theme that leads out of the template = %v", err)
	}

	// A template of one's own named in another case is refused, not passed over for the built-in one.
	cased := t.TempDir()
	own(t, cased, "Report")
	if _, err := draw(tmpl.NewLibrary(cased, ""), "template: report\n"); err == nil || !strings.Contains(err.Error(), "is not template report: template names are lower case") {
		t.Errorf("a template of one's own named Report = %v", err)
	}

	// A file a template may leave out is broken, not missing, when it is a symlink that leads nowhere.
	for _, file := range []string{"template.yaml", "checks.yaml", "computed.yaml"} {
		dir := t.TempDir()
		path := filepath.Join(own(t, dir, "report"), file)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		if err := os.Symlink("gone.yaml", path); err != nil {
			t.Fatal(err)
		}
		if _, err := draw(tmpl.NewLibrary(dir, ""), "template: report\n"); err == nil || !strings.Contains(err.Error(), file) {
			t.Errorf("a %s that leads nowhere = %v", file, err)
		}
	}

	// A directory of templates that cannot be read says so, rather than a template it may not hold.
	if os.Getuid() != 0 {
		locked := t.TempDir()
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(locked, 0o700); err != nil {
				t.Error(err)
			}
		})
		for _, src := range []string{"template: report\n", "title: no template\n"} {
			if _, err := draw(tmpl.NewLibrary(locked, ""), src); err == nil || !strings.Contains(err.Error(), "the directory of your own templates") {
				t.Errorf("%q under a directory of templates that cannot be read = %v", src, err)
			}
		}
	}
}

// template.yaml names the floe a template needs; a template for another major of floe, or for a later
// one, is refused before its data is checked.
func TestTemplateVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ yaml, want string }{
		{"floe: 0\n", ""},
		{"floe: 0.0.0\n", ""},
		{"floe: 1.2\n", "template report needs floe 1.2.0 or later, and this is floe 0.0.0"},
		{"floe: 0.1\n", "template report needs floe 0.1.0 or later, and this is floe 0.0.0"},
		{"floe: 0.0.1\n", "template report needs floe 0.0.1 or later"},
		{"floe: x\n", "/report/template.yaml:1:7: write template.yaml as one key, floe:"},
		{"floe: [0]\n", "template.yaml:1:7: write template.yaml as one key, floe:"},
		{"flow: 0\n", "template.yaml:1:1: flow is not a key of template.yaml"},
		{"", "template.yaml:1:1: write template.yaml as one key, floe:"},
	} {
		// Linked from the directory of templates, a mistake is named where it was found.
		dir := t.TempDir()
		at := own(t, t.TempDir(), "report")
		if err := os.Symlink(at, filepath.Join(dir, "report")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(at, "template.yaml"), []byte(tt.yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := draw(tmpl.NewLibrary(dir, ""), "template: report\nleed: a typo\n")
		var ue *usererr.Error
		switch {
		case tt.want == "" && (err == nil || !strings.Contains(err.Error(), "leed is not an item")):
			t.Errorf("template.yaml %q = %v, want the data's mistake", tt.yaml, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("template.yaml %q = %v, want %s", tt.yaml, err, tt.want)
		case strings.HasPrefix(tt.want, "/") && !strings.HasPrefix(err.Error(), dir):
			t.Errorf("template.yaml %q = %v, named where the symlink leads, not under %s", tt.yaml, err, dir)
		case tt.want != "" && !strings.Contains(tt.want, "template.yaml:") && !errors.As(err, &ue):
			t.Errorf("template.yaml %q = %T, want a usererr", tt.yaml, err)
		}
	}
}
