package tmpl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Library is the templates floe draws with: those of a directory of one's own, a directory each, then
// the built-in ones. One of one's own takes the place of a built-in one of its name.
type Library struct {
	dir string // "" without one
}

// NewLibrary returns the templates of dir, a directory of templates of one's own ("" for none), and
// the built-in ones.
func NewLibrary(dir string) *Library {
	return &Library{dir: dir}
}

// Template is a template found by name.
type Template struct {
	Name string
	// Dir is the real path of a template of one's own, a symlink followed; "" for a built-in one.
	Dir string
}

// Names are the templates there are, by name.
func (l *Library) Names() []string {
	var names []string
	if entries, err := fs.ReadDir(builtin, "builtin"); err == nil {
		for _, e := range entries {
			names = append(names, e.Name())
		}
	}
	if entries, err := os.ReadDir(l.dir); l.dir != "" && err == nil {
		for _, e := range entries {
			if t, err := l.own(e.Name()); err == nil && t != nil && !slices.Contains(names, e.Name()) {
				names = append(names, e.Name())
			}
		}
	}
	slices.Sort(names)
	return names
}

// Find returns the template name, or nil when there is none.
func (l *Library) Find(name string) (*Template, error) {
	if !validName(name) {
		return nil, nil //nolint:nilnil // not a name a template can have, so none has it
	}
	if t, err := l.own(name); t != nil || err != nil {
		return t, err
	}
	_, err := fs.Stat(builtin, path.Join("builtin", name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil //nolint:nilnil // no such template
	case err != nil:
		return nil, fmt.Errorf("find the template %s: %w", name, err)
	}
	return &Template{Name: name, Dir: ""}, nil
}

// own returns the template name of one's own, or nil without one. A file there is not a template, but
// a symlink that leads nowhere is a broken one: it does not give way to a built-in template.
func (l *Library) own(name string) (*Template, error) {
	if l.dir == "" || !validName(name) {
		return nil, nil //nolint:nilnil // none
	}
	at := filepath.Join(l.dir, name)
	if _, err := os.Lstat(at); errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // none
	}
	dir, err := filepath.EvalSymlinks(at)
	if err != nil {
		return nil, fmt.Errorf("find the template %s: %w", name, err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("find the template %s: %w", name, err)
	}
	if !fi.IsDir() {
		return nil, nil //nolint:nilnil // a file, not a template
	}
	return &Template{Name: name, Dir: dir}, nil
}

// validName reports whether name can name a template: a name of a directory, not a path, and not one
// starting with a dot, which floe keeps for the work of installing one.
func validName(name string) bool {
	return name != "" && filepath.IsLocal(name) && filepath.Base(name) == name && name[0] != '.' &&
		!strings.ContainsAny(name, `/\`)
}

// readFile reads the file name, a slash-separated path under the template. Under a template of one's
// own, a symlink that leads out of it is refused.
func (t *Template) readFile(name string) ([]byte, error) {
	var b []byte
	err := t.files(func(files fs.FS) error {
		var err error
		b, err = fs.ReadFile(files, name)
		return err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("read %s of the template %s: %w", name, t.Name, err)
	}
	return b, nil
}

// themes are the themes of t, by name.
func (t *Template) themes() ([]string, error) {
	var themes []string
	err := t.files(func(files fs.FS) error {
		entries, err := fs.ReadDir(files, "themes")
		for _, e := range entries {
			if name, ok := strings.CutSuffix(e.Name(), ".css"); ok && !e.IsDir() {
				themes = append(themes, name)
			}
		}
		return err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("read the themes of %s: %w", t.Name, err)
	}
	return themes, nil
}

// files gives use the files of t, as a file system rooted at t.
func (t *Template) files(use func(fs.FS) error) error {
	if t.Dir == "" {
		sub, err := fs.Sub(builtin, path.Join("builtin", t.Name))
		if err != nil {
			return fmt.Errorf("open the template %s: %w", t.Name, err)
		}
		return use(sub)
	}
	// os.Root refuses a path, a symlink included, that leads out of the template.
	root, err := os.OpenRoot(t.Dir)
	if err != nil {
		return fmt.Errorf("open the template %s: %w", t.Name, err)
	}
	defer root.Close()
	return use(root.FS())
}

// checkVersion refuses t when the floe it needs is not this one: template.yaml's floe: 1.2 takes a
// floe 1.2.0 or later of major 1. A built-in template, or one without template.yaml, needs none.
func (t *Template) checkVersion() error {
	if t.Dir == "" {
		return nil
	}
	src, err := t.readFile("template.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	needs, err := needsFloe(path.Join(t.Dir, "template.yaml"), src)
	if err != nil {
		return err
	}
	have, ok := versionOf(version.Version)
	if !ok {
		return fmt.Errorf("the version of floe %q is not major.minor.patch", version.Version) //nolint:err113 // floe's own
	}
	switch {
	case have[0] != needs[0]:
		return usererr.New("tmpl_floe_major", t.Name, needs[0], version.Version)
	case have[1] < needs[1] || have[1] == needs[1] && have[2] < needs[2]:
		return usererr.New("tmpl_floe_older", t.Name, needs.String(), version.Version)
	}
	return nil
}

// needsFloe reads the floe of template.yaml, file, as written: YAML would read 1.10 as the number 1.1.
func needsFloe(file string, src []byte) (semver, error) {
	body, err := yamlpos.Parse(file, src, "tmpl_template_yaml")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return semver{}, yamlpos.New(file, yamlpos.Pos{Line: 1, Column: 1}, "tmpl_template_yaml")
	}
	if err != nil {
		return semver{}, err //nolint:wrapcheck // a mistake at its line and column
	}
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return semver{}, yamlpos.New(file, yamlpos.Of(body), "tmpl_template_yaml")
	}
	var needs *semver
	for _, kv := range m.Values {
		key := kv.Key.GetToken().Value
		if key != "floe" {
			return semver{}, yamlpos.New(file, yamlpos.Of(kv.Key), "tmpl_template_yaml_key", key)
		}
		_, scalar := kv.Value.(ast.ScalarNode)
		v, ok := versionOf(kv.Value.GetToken().Value)
		if !scalar || !ok {
			return semver{}, yamlpos.New(file, yamlpos.OfValue(kv.Key, kv.Value), "tmpl_template_yaml")
		}
		needs = &v
	}
	if needs == nil {
		return semver{}, yamlpos.New(file, yamlpos.Of(body), "tmpl_template_yaml")
	}
	return *needs, nil
}

// semver is a version: major, minor and patch.
type semver [3]int

func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
}

// versionOf reads 1, 1.2 or 1.2.3, the parts left out being 0.
func versionOf(s string) (semver, bool) {
	var v semver
	parts := strings.Split(s, ".")
	if len(parts) > len(v) {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p != strconv.Itoa(n) {
			return v, false
		}
		v[i] = n
	}
	return v, true
}
