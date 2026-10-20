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
	return install(l.dir, name, "template", func(dir string) error { return src.copyTo(dir, name) })
}

// install makes the directory name, a template or a part (what), in parent of what fill writes into dir,
// and returns where it is. One there already is left as it is.
func install(parent, name, what string, fill func(dir string) error) (string, error) {
	at := filepath.Join(parent, name)
	switch _, lerr := os.Lstat(at); {
	case lerr == nil:
		return "", usererr.New("tmpl_new_exists", name, at)
	case !errors.Is(lerr, fs.ErrNotExist):
		return "", fmt.Errorf("make the %s %s: %w", what, name, lerr)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil { //nolint:gosec // G301: templates and parts are not secret
		return "", fmt.Errorf("make the %s %s: %w", what, name, err)
	}
	// Made under a name starting with a dot, which is no template nor part, and renamed when whole.
	tmp, err := os.MkdirTemp(parent, ".new-"+name+"-")
	if err != nil {
		return "", fmt.Errorf("make the %s %s: %w", what, name, err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // gone once renamed; what is left is a dot directory
	// MkdirTemp makes it 0700; a template or a part is no more secret than the directory it is in.
	if err = fill(tmp); err == nil {
		err = os.Chmod(tmp, 0o755) //nolint:gosec // G302: templates and parts are not secret
	}
	if err == nil {
		// os.Rename refuses a directory there, so one made since the check above is not written over.
		err = os.Rename(tmp, at)
	}
	if errors.Is(err, fs.ErrExist) {
		return "", usererr.New("tmpl_new_exists", name, at)
	}
	if err != nil {
		return "", fmt.Errorf("make the %s %s: %w", what, name, err)
	}
	return at, nil
}

// copyTo writes the files of t into dir as the template name.
func (t *Template) copyTo(dir, name string) error {
	if err := t.files(func(files fs.FS) error { return os.CopyFS(dir, files) }); err != nil {
		return err
	}
	// A built-in template comes with this floe, which a copy of it names; a copy of one of one's own
	// keeps its template.yaml, or its lack of one, which must read.
	if _, err := t.needs(); err != nil {
		return err
	}
	if t.Dir == "" {
		have, err := thisFloe()
		if err != nil {
			return err
		}
		floe := "floe: " + have.minor() + "\n"
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
		// After the colon that ends the key, written bare or in quotes.
		from = len(line)
		if i := strings.IndexRune(string(line[k.Column-1:]), ':'); i >= 0 {
			from = k.Column + len([]rune(string(line[k.Column-1:])[:i]))
		}
	}
	head, tail := string(line[:from]), string(line[from:])
	if i := strings.Index(tail, old); i >= 0 && old != "" {
		lines[v.Line-1] = head + tail[:i] + name + tail[i+len(old):]
	}
	return []byte(strings.Join(lines, ""))
}
