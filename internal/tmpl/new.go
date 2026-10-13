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
// It names the floe the copy needs in template.yaml, the one from names or else this floe, and the
// copy in example.yaml's template:. A template of one's own already named so is left as it is.
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
	needs, err := t.needs()
	if err != nil {
		return err
	}
	if needs == nil {
		have, _ := versionOf(version.Version)
		floe := fmt.Sprintf("floe: %d.%d\n", have[0], have[1])
		//nolint:gosec // G306: templates are not secret
		if err = os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(floe), 0o644); err != nil {
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
	at := yamlpos.OfValue(key, value)
	old := value.GetToken().Value
	lines := strings.SplitAfter(string(src), "\n")
	// The value starts at its column, counted in characters, after a quote if it has one.
	line := []rune(lines[at.Line-1])
	head, tail := string(line[:at.Column-1]), string(line[at.Column-1:])
	if i := strings.Index(tail, old); i >= 0 && old != "" {
		lines[at.Line-1] = head + tail[:i] + name + tail[i+len(old):]
	}
	return []byte(strings.Join(lines, ""))
}
