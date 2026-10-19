package tmpl

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Library is the templates floe draws with: those of a directory of one's own, a directory each, then
// the built-in ones. One of one's own takes the place of a built-in one of its name.
type Library struct {
	dir   string // "" without one
	icons string // the directory of icons the diagrams of data files show; "" for none
}

// NewLibrary returns the templates of dir, a directory of templates of one's own ("" for none), and
// the built-in ones, which draw the icons kept in iconsDir (FLOE_ICONS_DIR) in diagrams.
func NewLibrary(dir, iconsDir string) *Library {
	return &Library{dir: dir, icons: iconsDir}
}

// Dir is the directory of templates of one's own, "" for none.
func (l *Library) Dir() string {
	return l.dir
}

// IconsDir is the directory of icons the diagrams of data files show.
func (l *Library) IconsDir() string {
	return l.icons
}

// Template is a template found by name.
type Template struct {
	Name string
	// Dir is the real path of a template of one's own, a symlink followed; "" for a built-in one.
	Dir string
	// Path is where a template of one's own was found, as its mistakes name it; "" for a built-in one.
	Path string

	icons string // the directory of icons of the library it was found in
}

// Names are the templates there are, by name. It fails when the directory of templates of one's own
// cannot be read, rather than leave them out.
func (l *Library) Names() ([]string, error) {
	var names []string
	if entries, err := fs.ReadDir(templateFiles, "templates"); err == nil {
		for _, e := range entries {
			names = append(names, e.Name())
		}
	}
	own, err := l.ownNames()
	if err != nil {
		return nil, err
	}
	for _, name := range own {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// ownNames are the templates of one's own, by name: none without a directory of them. One that is
// broken (a symlink that leads nowhere) is named too, for its mistake to be reported where it is used.
func (l *Library) ownNames() ([]string, error) {
	if l.dir == "" {
		return nil, nil
	}
	entries, err := l.entries()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !validName(e.Name()) {
			continue
		}
		if t, err := l.resolve(e.Name()); err != nil || t != nil {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// entries are what the directory of templates of one's own holds: none without one, or with a file in
// its way.
// LIMIT: read on every lookup. Keep the entries if a directory comes to hold thousands of them.
func (l *Library) entries() ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(l.dir)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return nil, nil
	case err != nil:
		return nil, usererr.New("tmpl_dir_unreadable", l.dir, err.Error())
	}
	return entries, nil
}

// unknown refuses the template name, which there is none of, naming those there are.
func (l *Library) unknown(name string) error {
	names, err := l.Names()
	if err != nil {
		return err
	}
	return usererr.New("tmpl_unknown", name, strings.Join(names, ", "))
}

// Find returns the template name, or nil when there is none.
func (l *Library) Find(name string) (*Template, error) {
	if !validName(name) {
		return nil, nil //nolint:nilnil // not a name a template can have, so none has it
	}
	if t, err := l.own(name); t != nil || err != nil {
		return t, err
	}
	_, err := fs.Stat(templateFiles, path.Join("templates", name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil //nolint:nilnil // no such template
	case err != nil:
		return nil, fmt.Errorf("find the template %s: %w", name, err)
	}
	return &Template{Name: name, Dir: "", Path: "", icons: l.icons}, nil
}

// own returns the template name of one's own, or nil without one. A file there is not a template, but
// a symlink that leads nowhere is a broken one: it does not give way to a built-in template.
func (l *Library) own(name string) (*Template, error) {
	if l.dir == "" || !validName(name) {
		return nil, nil //nolint:nilnil // none
	}
	entries, err := l.entries()
	if err != nil {
		return nil, err
	}
	// The entry of the name as written: a file system that ignores case (macOS) would take Report for
	// report. One of another case is refused rather than passed over for a built-in template.
	if slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return e.Name() == name }) {
		return l.resolve(name)
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return nil, usererr.New("tmpl_own_case", filepath.Join(l.dir, e.Name()), name)
		}
	}
	return nil, nil //nolint:nilnil // none
}

// resolve returns the template of one's own at the entry name, or nil when it is a file.
func (l *Library) resolve(name string) (*Template, error) {
	at := filepath.Join(l.dir, name)
	dir, err := filepath.EvalSymlinks(at)
	if err != nil {
		return nil, usererr.New("tmpl_own_broken", name, at, err.Error())
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, usererr.New("tmpl_own_broken", name, at, err.Error())
	}
	if !fi.IsDir() {
		return nil, nil //nolint:nilnil // a file, not a template
	}
	return &Template{Name: name, Dir: dir, Path: at, icons: l.icons}, nil
}

// validName reports whether name can name a template: lower case letters, digits, - and _, so that it
// is not a path, nor a name starting with a dot, which floe keeps for the work of installing one, and
// so that it names the same template on a file system that ignores case (macOS).
func validName(name string) bool {
	return templateName.MatchString(name)
}

var templateName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// file names the file name of t in its mistakes: under where t was found, or under its name for a
// built-in one.
func (t *Template) file(name string) string {
	return filepath.Join(cmp.Or(t.Path, t.Name), name)
}

// parts returns the parts t is drawn with and their themes.
func (*Template) parts() (*partSet, error) {
	return builtinParts()
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

// readOptional reads the file name like readFile, and found is false when t has none. A symlink that
// leads nowhere is a broken file, not a missing one, and a template gone is an error.
func (t *Template) readOptional(name string) (b []byte, found bool, err error) {
	err = t.files(func(files fs.FS) error {
		if _, statErr := fs.Lstat(files, name); errors.Is(statErr, fs.ErrNotExist) {
			return nil
		}
		found = true
		var readErr error
		b, readErr = fs.ReadFile(files, name)
		return readErr //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, false, fmt.Errorf("read %s of the template %s: %w", name, t.Name, err)
	}
	return b, found, nil
}

// themes are the themes of t, by name.
func (t *Template) themes() ([]string, error) {
	var themes []string
	err := t.files(func(files fs.FS) error {
		var err error
		themes, err = themesIn(files, "themes")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("read the themes of %s: %w", t.Name, err)
	}
	return themes, nil
}

// themesIn returns the themes in the directory dir of files, its CSS files, by name.
func themesIn(files fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(files, dir)
	var themes []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".css"); ok && !e.IsDir() {
			themes = append(themes, name)
		}
	}
	return themes, err //nolint:wrapcheck // wrapped by the callers
}

// files calls read with the files of t, as a file system rooted at t.
func (t *Template) files(read func(fs.FS) error) error {
	if t.Dir == "" {
		sub, err := fs.Sub(templateFiles, path.Join("templates", t.Name))
		if err != nil {
			return fmt.Errorf("open the template %s: %w", t.Name, err)
		}
		return read(sub)
	}
	// os.Root refuses a path, a symlink included, that leads out of the template.
	root, err := os.OpenRoot(t.Dir)
	if err != nil {
		return fmt.Errorf("open the template %s: %w", t.Name, err)
	}
	defer root.Close()
	return read(root.FS())
}

// checkVersion refuses t when the floe it needs is not this one: template.yaml's floe: 1.2 takes a
// floe 1.2.0 or later of major 1, and floe: 0.2 one of 0.2, as a minor version of 0 may break what the
// one before it took. A built-in template, or one without template.yaml, needs none.
func (t *Template) checkVersion() error {
	needs, err := t.needs()
	if needs == nil || err != nil {
		return err
	}
	have, err := thisFloe()
	if err != nil {
		return err
	}
	return fits(t.Name, *needs, have)
}

// thisFloe is the version of this floe, a pre-release or build after it (1.2.3-rc.1) left out: a
// template written for 1.2.3 is drawn by its release candidates too.
func thisFloe() (semver, error) {
	v, ok := releaseOf(version.Version)
	if !ok {
		//nolint:err113 // floe's own version, set when it is built
		return v, fmt.Errorf("the version of this floe %q is not a version like 1.2.3", version.Version)
	}
	return v, nil
}

// needs returns the floe t needs, or nil when it names none.
func (t *Template) needs() (*semver, error) {
	if t.Dir == "" {
		return nil, nil //nolint:nilnil // a built-in template needs this floe, which it comes with
	}
	src, found, err := t.readOptional("template.yaml")
	if !found || err != nil {
		return nil, err
	}
	needs, err := needsFloe(t.file("template.yaml"), src)
	if err != nil {
		return nil, err
	}
	return &needs, nil
}

// fits refuses the template name, which needs a floe of needs, for a floe of have.
func fits(name string, needs, have semver) error {
	switch {
	case slices.Compare(have[:], needs[:]) < 0:
		return usererr.New("tmpl_floe_older", name, needs.String(), have.String())
	case have.series() != needs.series():
		return usererr.New("tmpl_floe_major", name, needs.series(), have.String())
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
	if err = plain(file, body); err != nil {
		return semver{}, err
	}
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return semver{}, yamlpos.New(file, yamlpos.Of(body), "tmpl_template_yaml")
	}
	var needs *semver
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		key := k.GetToken().Value
		if key != "floe" {
			return semver{}, yamlpos.New(file, yamlpos.Of(k), "tmpl_template_yaml_key", key)
		}
		_, scalar := kv.Value.(ast.ScalarNode)
		v, ok := versionOf(kv.Value.GetToken().Value)
		if !scalar || !ok {
			return semver{}, yamlpos.New(file, yamlpos.OfValue(k, kv.Value), "tmpl_template_yaml")
		}
		needs = &v
	}
	if needs == nil {
		return semver{}, yamlpos.New(file, yamlpos.Of(body), "tmpl_template_yaml")
	}
	return *needs, nil
}

// releaseOf reads the version s, a pre-release or build after it left out.
func releaseOf(s string) (semver, bool) {
	core, _, _ := strings.Cut(s, "-")
	core, _, _ = strings.Cut(core, "+")
	return versionOf(core)
}

// semver is a version: major, minor and patch.
type semver [3]int

func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
}

// minor writes v as major.minor, as template.yaml names the floe a template needs.
func (v semver) minor() string {
	return fmt.Sprintf("%d.%d", v[0], v[1])
}

// series is the versions that take what v takes: its major, or its minor under major 0.
func (v semver) series() string {
	if v[0] == 0 {
		return fmt.Sprintf("0.%d", v[1])
	}
	return strconv.Itoa(v[0])
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
