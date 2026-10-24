package tmpl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

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
	case errors.Is(lerr, fs.ErrPermission):
		return "", usererr.CannotWrite(at, lerr)
	case !errors.Is(lerr, fs.ErrNotExist):
		return "", fmt.Errorf("make the %s %s: %w", what, name, lerr)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil { //nolint:gosec // G301: templates and parts are not secret
		return "", usererr.CannotWrite(at, err)
	}
	// Made under a name starting with a dot, which is no template nor part, and renamed when whole.
	tmp, err := os.MkdirTemp(parent, ".new-"+name+"-")
	if err != nil {
		return "", usererr.CannotWrite(at, err)
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
	// Read as written, a comment on a line too: the copy names itself all the same, and its check tells
	// the rest.
	root, err := yamlpos.Document("example.yaml", src, "tmpl_documents")
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

// MadePart is what NewPart made.
type MadePart struct {
	At       string   // the part's directory
	Builtin  bool     // whether it is a copy of floe's
	Replaces bool     // whether it takes the place of floe's of its name
	Themes   []string // the themes it has a file of, as copies of the original's, by name
	ThemesAt string   // the directory of the themes of one's own those files are under
}

// NewPart makes the part name of one's own as a copy of the part from, floe's or one's own, with a
// copy of the file of from of each theme that has one, and tells of what it made. The box of the
// copy names it; what else its part.html names is kept. Nothing is written over: with a file of a
// theme for name there already, nothing is made, nor is a copy that reading the parts would leave out,
// which is read in its place before it is put there.
// LIMIT: a symlink in a part of one's own is not copied (os.CopyFS refuses it), nor is a data-floe-part
// that part.html puts together ({{…}}) renamed, which the check of its names then refuses. Copy what the
// link leads to, or follow the template, if a part of one's own comes to need that.
func (l *Library) NewPart(name, from string) (*MadePart, error) {
	if l.mine.parts == "" {
		return nil, usererr.New("tmpl_no_parts_dir")
	}
	if !validName(name) {
		return nil, usererr.New("tmpl_new_part_name", name)
	}
	set, err := partsIn(l.mine)
	if err != nil {
		return nil, err
	}
	if set.reserved(name) {
		return nil, usererr.New("tmpl_new_part_reserved", name)
	}
	src, err := set.find(from)
	if err != nil {
		return nil, err
	}
	if isGoPart(from) {
		return nil, usererr.New("tmpl_new_part_go", from)
	}
	floe, err := builtinParts()
	if err != nil {
		return nil, err
	}
	made := &MadePart{
		At: "", Builtin: src.dir == "", Replaces: floe.parts[name] != nil, Themes: set.themesWith(from),
		ThemesAt: l.mine.themes,
	}
	if made.Themes != nil && made.ThemesAt == "" {
		return nil, usererr.New("tmpl_no_themes_dir")
	}
	themes := &themeCopies{written: nil, made: nil}
	made.At, err = install(l.mine.parts, name, "part", func(dir string) error {
		if cerr := set.checkThemeFiles(floe, src, made, name); cerr != nil {
			return cerr
		}
		if cerr := src.copyTo(dir, name); cerr != nil {
			return cerr
		}
		// Before the part is renamed into place, so that a theme that cannot be written leaves nothing: a
		// page drawn meanwhile is warned of the files, of no part yet.
		if cerr := themes.write(set, made, from, name); cerr != nil {
			return cerr
		}
		// Read as a page would read it in place, which may leave it out, or others with it (a look in place
		// of one of floe's that draws, a part that holds itself): then it is not put in place.
		return set.unusableWith(l.mine, name, dir)
	})
	if err != nil {
		themes.undo()
		return nil, err
	}
	return made, nil
}

// checkThemeFiles refuses to copy the part src as name, made, when a theme of one's own has a file
// of name already, whether it has a file of src or not, or when one that cannot be used may have a file of
// src, which the copy would then lack: of its own, read or not, or of floe's theme of its name for a part
// of floe's. The themes of one's own that cannot be read at all are such.
func (set *partSet) checkThemeFiles(floe *partSet, src *part, made *MadePart, name string) error {
	if made.ThemesAt == "" {
		return nil // no theme of one's own, nor a file of one
	}
	if _, err := os.Lstat(made.ThemesAt); err == nil {
		if dir, derr := dirAt(made.ThemesAt); dir == "" {
			why := "not a directory"
			if derr != nil {
				why = derr.Error()
			}
			return usererr.NewWhole("tmpl_themes_dir_unreadable", made.ThemesAt, why)
		}
	}
	var there []string
	for _, theme := range set.themeNames() {
		dir := filepath.Join(made.ThemesAt, theme)
		if set.lacks(floe, src, theme, dir) {
			return set.brokenTheme(theme)
		}
		at := filepath.Join(dir, name+".css")
		switch _, err := os.Lstat(at); {
		case err == nil:
			there = append(there, at)
		case errors.Is(err, fs.ErrPermission):
			return usererr.New("tmpl_new_part_theme", usererr.CannotWrite(at, err))
		case !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR):
			return fmt.Errorf("look for %s: %w", at, err)
		}
	}
	if there != nil {
		return usererr.New("tmpl_new_part_theme_exists", name, strings.Join(there, ", "))
	}
	return nil
}

// lacks reports whether the theme of one's own theme, in dir, cannot be used and may have a file of
// the part src, of its own, read or not, or of floe's theme of its name for a part of floe's.
func (set *partSet) lacks(floe *partSet, src *part, theme, dir string) bool {
	if set.brokenThemes[theme] == nil {
		return false
	}
	_, unread := dirAt(dir)
	_, own := os.Lstat(filepath.Join(dir, src.name+".css"))
	floes := false
	if t := floe.themes[theme]; t != nil {
		_, floes = t.parts[src.name]
	}
	return unread != nil || own == nil || src.dir == "" && floes
}

// unusableWith refuses the part name, made in the directory dir to be put in parts, when reading parts with
// it in place leaves out a part or a theme that set, read before, did not: the copy, or one that
// takes its look or holds it.
func (set *partSet) unusableWith(own ownDirs, name, dir string) error {
	after, err := partsWith(own, name, dir)
	if err != nil {
		return err
	}
	// The copy first, then those it leaves out with it.
	var found usererr.List
	if after.broken[name] != nil {
		found = append(found, after.brokenPart(name))
	}
	found = slices.Concat(found,
		after.unusable(func(n string) bool { return n == name || set.broken[n] != nil }),
		after.unusableThemes(func(n string) bool { return set.brokenThemes[n] != nil }))
	if found != nil {
		return usererr.Over("", found, "tmpl_new_part_unusable", name)
	}
	return nil
}

// copyTo writes the files of p into dir as the part name, its box renamed.
func (p *part) copyTo(dir, name string) error {
	if err := p.files(func(files fs.FS) error { return os.CopyFS(dir, files) }); err != nil {
		return err
	}
	if p.isLook() || name == p.name {
		return nil
	}
	html := filepath.Join(dir, "part.html")
	//nolint:gosec // G306: parts are not secret
	return os.WriteFile(html, []byte(renamedPart(p.html, p.name, name)), 0o644) //nolint:wrapcheck // wrapped by install
}

// files calls read with the files of p, as a file system rooted at its directory.
func (p *part) files(read func(fs.FS) error) error {
	if p.dir == "" {
		sub, err := fs.Sub(partFiles, path.Join("parts", p.name))
		if err != nil {
			return fmt.Errorf("open the part %s: %w", p.name, err)
		}
		return read(sub)
	}
	dir, err := dirAt(p.dir)
	if err != nil {
		return fmt.Errorf("open the part %s: %w", p.name, err)
	}
	return readIn(dir, read)
}

// renamedPart returns html, a part's, its box named name in place of from: the value of each
// data-floe-part that is from as written, of a tag. What else names from, a look or text, is kept.
func renamedPart(html, from, name string) string {
	var b strings.Builder
	last := 0
	for _, t := range tagsIn(html, true) {
		for _, a := range t.attrs {
			if t.end || a.name != "data-floe-part" || a.value != from {
				continue
			}
			at := a.valueAt
			if html[at] == '"' || html[at] == '\'' {
				at++
			}
			if !strings.HasPrefix(html[at:], from) {
				continue
			}
			b.WriteString(html[last:at] + name)
			last = at + len(from)
		}
	}
	return b.String() + html[last:]
}

// themeCopies are the files of themes NewPart wrote, and the directories it made for them, to be
// removed when the part is not made.
type themeCopies struct {
	written []string
	made    []string
}

// write writes the file of the part name of each theme of made, a copy of the theme's file of from, in
// the theme of its name of one's own, refusing one written since checkThemeFiles.
func (w *themeCopies) write(set *partSet, made *MadePart, from, name string) error {
	for _, theme := range made.Themes {
		at := filepath.Join(made.ThemesAt, theme, name+".css")
		if err := w.writeFile(made.ThemesAt, at, set.themes[theme].parts[from]); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return usererr.New("tmpl_new_part_theme_exists", name, at)
			}
			return usererr.New("tmpl_new_part_theme", usererr.CannotWrite(at, err))
		}
	}
	return nil
}

// writeFile writes css into the file at, a new one, of a theme in themes, the directory of themes of one's
// own, making the directories it needs, those above themes among them.
// LIMIT: a file system without hard links refuses the file, and so the copy. Create it with O_EXCL and
// write it in place there if one comes to be used.
func (w *themeCopies) writeFile(themes, at, css string) error {
	var above []string
	for dir := filepath.Dir(themes); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		above = append(above, dir)
	}
	slices.Reverse(above)
	for _, dir := range append(above, themes, filepath.Dir(at)) {
		switch err := os.Mkdir(dir, 0o755); { //nolint:gosec // G301: themes are not secret
		case err == nil:
			w.made = append(w.made, dir)
		case !errors.Is(err, fs.ErrExist):
			return err //nolint:wrapcheck // told by write
		}
	}
	// Written whole under a name starting with a dot, which is no theme's file, and linked into place.
	tmp, err := os.CreateTemp(filepath.Dir(at), ".new-*.css")
	if err != nil {
		return err //nolint:wrapcheck // told by write
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // a dot file left is no theme's
	_, err = tmp.WriteString(css)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644) //nolint:gosec // G302: themes are not secret
	}
	if err == nil {
		// os.Link refuses a file there, so one written since the check is not written over.
		err = os.Link(tmp.Name(), at)
	}
	if err != nil {
		return err //nolint:wrapcheck // told by write
	}
	w.written = append(w.written, at)
	return nil
}

// undo removes the files w wrote, and the directories it made, which nothing else holds.
func (w *themeCopies) undo() {
	for _, at := range w.written {
		_ = os.Remove(at) //nolint:errcheck // what is left is told as a file of no part
	}
	for _, dir := range slices.Backward(w.made) {
		_ = os.Remove(dir) //nolint:errcheck // one that holds files is kept
	}
}
