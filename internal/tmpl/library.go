package tmpl

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Library is the templates floe draws with: those of a directory of one's own, a directory each, then
// the built-in ones. One of one's own takes the place of a built-in one of its name.
type Library struct {
	dir   string  // "" without one
	mine  ownDirs // the directories of parts and themes of one's own
	icons string  // the directory of icons the diagrams of data files show; "" for none
	run   *run    // what one output read once (Run); nil reads it again every time
}

// run is what the reads of one output read once: the parts and themes of one's own, the directory of
// templates of one's own, the templates found by name and the icons. Each mistake of them is then the same
// for every input that uses what it is of, and told once. It is for one output, in one goroutine: kept for
// longer, it would not see the files fixed after.
type run struct {
	own       ownDirs
	set       *partSet
	setErr    error
	setRead   bool
	entries   map[string]entriesRead // by the directory of templates
	templates map[templateAt]found
	icons     *icons.Store
}

// templateAt is a template by the directory of templates it was looked up in and its name.
type templateAt struct{ dir, name string }

type entriesRead struct {
	entries []fs.DirEntry
	err     error
}

type found struct {
	t   *Template
	err error
}

// Run returns l, reading what its reads share once from now on: for one output (a command's run, a
// server's answer to a request), never for longer, as the files may be fixed in between.
func (l *Library) Run() *Library {
	r := &run{
		own: l.mine, set: nil, setErr: nil, setRead: false, entries: map[string]entriesRead{},
		templates: map[templateAt]found{}, icons: icons.New(l.icons),
	}
	return &Library{dir: l.dir, mine: l.mine, icons: l.icons, run: r}
}

// parts reads the parts and themes of one's own of r, once.
func (r *run) parts() (*partSet, error) {
	if !r.setRead {
		r.set, r.setErr = partsIn(r.own)
		r.setRead = true
	}
	return r.set, r.setErr
}

// partsRead returns the parts and themes of one's own of l: once in a run, else read anew.
func (l *Library) partsRead() (*partSet, error) {
	if l.run != nil {
		return l.run.parts()
	}
	return partsIn(l.mine)
}

// NewLibrary returns the templates of dir, a directory of templates of one's own ("" for none), and
// the built-in ones, which draw with floe's parts and themes and those of partsDir (FLOE_PARTS_DIR) and
// themesDir (FLOE_THEMES_DIR), "" for none, and the icons kept in iconsDir (FLOE_ICONS_DIR) in diagrams.
func NewLibrary(dir, partsDir, themesDir, iconsDir string) *Library {
	return &Library{dir: dir, mine: ownDirs{parts: partsDir, themes: themesDir}, icons: iconsDir, run: nil}
}

// PartsDir is the directory of parts of one's own, "" for none.
func (l *Library) PartsDir() string {
	return l.mine.parts
}

// ThemesDir is the directory of themes of one's own, "" for none.
func (l *Library) ThemesDir() string {
	return l.mine.themes
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

	icons string   // the directory of icons of the library it was found in
	own   ownDirs  // the directories of parts and themes of one's own of that library
	run   *run     // what the run it was found in read once; nil without one
	set   *partSet // the parts it is drawn with, once read
	reads *reads
}

// reads is what each file of a template gives, read once: every data file drawn with it gets the same,
// and the same mistake, to be told once. What a data file gives is computed from them for each.
type reads struct {
	version  readOnce[struct{}]
	words    readOnce[words]
	page     readOnce[parsedPage]
	sheet    readOnce[*cssSheet] // page.css
	shape    readOnce[schemaRead]
	computed readOnce[*computed] // nil without computed.yaml
	checks   readOnce[[]relation]
}

// readOnce is what reading a file gave, read the first time alone.
type readOnce[T any] struct {
	done bool
	v    T
	err  error
}

// of returns what read gave the first time it was called.
func (r *readOnce[T]) of(read func() (T, error)) (T, error) {
	if !r.done {
		r.v, r.err = read()
		r.done = true
	}
	return r.v, copied(r.err)
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
// its way. A run reads them once.
// LIMIT: out of a run, read on every lookup. Keep them if a directory comes to hold thousands of them.
func (l *Library) entries() ([]fs.DirEntry, error) {
	if l.run != nil {
		if e, ok := l.run.entries[l.dir]; ok {
			return e.entries, e.err
		}
	}
	entries, err := entriesOf(l.dir)
	var e entriesRead
	if err != nil {
		e.err = usererr.New("tmpl_dir_unreadable", l.dir, err.Error())
	} else {
		e.entries = entries
	}
	if l.run != nil {
		l.run.entries[l.dir] = e
	}
	return e.entries, e.err
}

// unknown refuses the template name, which there is none of, naming those there are.
func (l *Library) unknown(name string) error {
	names, err := l.Names()
	if err != nil {
		return err
	}
	return usererr.New("tmpl_unknown", name, strings.Join(names, ", "))
}

// Find returns the template name, or nil when there is none: in a run, the same one each time.
func (l *Library) Find(name string) (*Template, error) {
	if l.run == nil {
		return l.lookUp(name)
	}
	at := templateAt{dir: l.dir, name: name}
	f, ok := l.run.templates[at]
	if !ok {
		f.t, f.err = l.lookUp(name)
		l.run.templates[at] = f
	}
	return f.t, f.err
}

func (l *Library) lookUp(name string) (*Template, error) {
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
	return l.template(name, "", ""), nil
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
	dir, err := dirAt(at)
	if err != nil {
		return nil, usererr.New("tmpl_own_broken", name, at, err.Error())
	}
	if dir == "" {
		return nil, nil //nolint:nilnil // a file, not a template
	}
	return l.template(name, dir, at), nil
}

func (l *Library) template(name, dir, at string) *Template {
	return &Template{
		Name: name, Dir: dir, Path: at, icons: l.icons, own: l.mine, run: l.run, set: nil, reads: new(reads),
	}
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

// isOwn reports whether t is one's own, not built in: a file of it that fails on data its schema took is its
// maker's mistake, else floe's failure.
func (t *Template) isOwn() bool {
	return t.Dir != ""
}

// parts returns the parts t is drawn with and their themes: those its run read once, else read for t.
func (t *Template) parts() (*partSet, error) {
	if t.set == nil {
		var set *partSet
		var err error
		if t.run != nil {
			set, err = t.run.parts()
		} else {
			set, err = partsIn(t.own)
		}
		if err != nil {
			return nil, err
		}
		t.set = set
	}
	return t.set, nil
}

// iconStore is the icons t's diagrams show: those its run reads once, else read anew.
func (t *Template) iconStore() *icons.Store {
	if t.run != nil {
		return t.run.icons
	}
	return icons.New(t.icons)
}

// copied is err, a List of it copied: those it is given to may add to it.
func copied(err error) error {
	if l, ok := err.(usererr.List); ok { //nolint:errorlint // the List itself, not one it wraps
		return slices.Clone(l) //nolint:wrapcheck // the same mistakes
	}
	return err
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
		return nil, usererr.CannotRead(t.file(name), err)
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
		return nil, false, usererr.CannotRead(t.file(name), err)
	}
	return b, found, nil
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
	return readIn(t.Dir, read)
}

// checkVersion refuses t when the floe it needs is not this one: template.yaml's floe: 1.2 takes a
// floe 1.2.0 or later of major 1, and floe: 0.2 one of 0.2, as a minor version of 0 may break what the
// one before it took. A built-in template, or one without template.yaml, needs none.
func (t *Template) checkVersion() error {
	_, err := t.reads.version.of(func() (struct{}, error) {
		needs, err := t.needs()
		if needs == nil || err != nil {
			return struct{}{}, err
		}
		have, err := thisFloe()
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, fits(t.Name, *needs, have)
	})
	return err
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
	m, err := readTop[*ast.MappingNode](file, body, false, "tmpl_template_yaml")
	if err != nil {
		return semver{}, err
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
