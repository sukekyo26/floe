package tmpl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// New makes the template name of one's own as a copy of the template from, and returns where it is.
// A copy of a built-in template names this floe in template.yaml, and the copy names itself in
// example.yaml's template:. A template of one's own already named so is left as it is.
// LIMIT: a symlink in from is not copied (os.CopyFS refuses it). Copy what it leads to if a template
// of one's own comes to need that.
func (l *Library) New(name, from string) (string, error) {
	if l.dir == "" {
		return "", usererr.New("tmpl_no_dir")
	}
	if !validName(name) {
		return "", usererr.New("tmpl_new_name", name)
	}
	src, err := l.Find(from)
	if err != nil {
		return "", err
	}
	if src == nil {
		return "", l.unknown(from)
	}
	at := filepath.Join(l.dir, name)
	switch _, lerr := os.Lstat(at); {
	case lerr == nil:
		return "", usererr.New("tmpl_new_exists", name, at)
	case !errors.Is(lerr, fs.ErrNotExist):
		return "", fmt.Errorf("make the template %s: %w", name, lerr)
	}
	if err = os.MkdirAll(l.dir, 0o755); err != nil { //nolint:gosec // G301: templates are not secret
		return "", fmt.Errorf("make the template %s: %w", name, err)
	}
	// Made under a name starting with a dot, which is no template, and renamed when whole.
	tmp, err := os.MkdirTemp(l.dir, ".new-"+name+"-")
	if err != nil {
		return "", fmt.Errorf("make the template %s: %w", name, err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // gone once renamed; what is left is a dot directory
	// MkdirTemp makes it 0700; a template is no more secret than the directory of templates.
	if err = src.copyTo(tmp, name); err == nil {
		err = os.Chmod(tmp, 0o755) //nolint:gosec // G302: templates are not secret
	}
	if err == nil {
		err = os.Rename(tmp, at)
	}
	if err != nil {
		return "", fmt.Errorf("make the template %s: %w", name, err)
	}
	return at, nil
}

// copyTo writes the files of t into dir as the template name.
func (t *Template) copyTo(dir, name string) error {
	if err := t.files(func(files fs.FS) error { return os.CopyFS(dir, files) }); err != nil {
		return err
	}
	// A built-in template comes with this floe, which a copy of it names; a copy of one of one's own
	// keeps its template.yaml, or its lack of one.
	if t.Dir == "" {
		have, ok := versionOf(version.Version)
		if !ok {
			return fmt.Errorf("the version of floe %q is not major.minor.patch", version.Version) //nolint:err113 // floe's own
		}
		floe := fmt.Sprintf("floe: %d.%d\n", have[0], have[1])
		//nolint:gosec // G306: templates are not secret
		if err := os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(floe), 0o644); err != nil {
			return err //nolint:wrapcheck // wrapped by New
		}
	}
	example := filepath.Join(dir, "example.yaml")
	src, err := os.ReadFile(example)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err //nolint:wrapcheck // wrapped by New
	}
	return os.WriteFile(example, renamed(src, name), 0o644) //nolint:gosec,wrapcheck // G306: not secret; wrapped by New
}

// renamed returns the data file src naming the template name, its other text as it was: the value of
// its top-level template: is written over. A file that does not read as one names none to rename.
func renamed(src []byte, name string) []byte {
	root, err := parse("example.yaml", src)
	if err != nil {
		return src
	}
	key, value, path := yamlpos.Find(root, []string{"template"})
	if _, text := value.(*ast.StringNode); key == nil || path != "template" || !text {
		return src
	}
	old := value.GetToken().Value
	k, v := yamlpos.Of(key), yamlpos.Of(value)
	lines := strings.SplitAfter(string(src), "\n")
	// The value is the first old after the key on its line, or on a line of its own. goccy/go-yaml
	// moves a value's column right by the blanks after it, so the column is not where to look.
	line := []rune(lines[v.Line-1])
	from := 0
	if v.Line == k.Line {
		from = min(k.Column-1+len([]rune("template")), len(line))
	}
	head, tail := string(line[:from]), string(line[from:])
	if i := strings.Index(tail, old); i >= 0 && old != "" {
		lines[v.Line-1] = head + tail[:i] + name + tail[i+len(old):]
	}
	return []byte(strings.Join(lines, ""))
}
