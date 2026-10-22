// Package tmpl draws pages from data files: a YAML file names a template, and the template's page.html
// lays its data out. The templates are built in (report), or of one's own. A page is drawn in one of floe's
// themes, which gives its palette and the parts' shape, and the template's page.css lays the page out.
package tmpl

import (
	"bytes"
	"cmp"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// templateFiles holds a directory per built-in template: page.html, schema.json, example.yaml and page.css.
//
//go:embed templates
var templateFiles embed.FS

const (
	// DefaultTheme is the theme of a page whose data names none.
	DefaultTheme = "default"
)

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// markdownLine reads paragraphs alone, without the paragraph transformers that make a line a table or a
// link reference definition, so that a line like "-", "1." or "# x" stays text.
var markdownLine = goldmark.New(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
	)),
	goldmark.WithExtensions(extension.Strikethrough, extension.Linkify),
)

// Data is a data file read: the keys every template shares, and the rest for the template's schema.
type Data struct {
	File     string
	Template string
	Theme    string // the theme it is drawn with: its palette and the shape of the parts
	Title    string // "" when the data names none
	Lang     string // the language of the page, a BCP 47 tag
	Values   map[string]any
	Calc     map[string]any // the values of the template's computed.yaml, by name; nil without one

	tmpl   *Template               // the template Read checked the data against, which draws it
	shape  *shape                  // the shape of the template's schema as Read read it, for drawing as checked
	images map[string]template.URL // its images as data URIs, by the path written
	// its items drawn by floe's parts, by the path of keys to each (pathKey); nil without one
	parts     map[string]template.HTML
	diagrams  bool       // whether it has a diagram, for the page to get their style and zoom
	tried     []string   // the images it names that a file can make right, read or not, as it writes them
	icons     []string   // the index files of the icon sets its diagrams may show, when it has one
	htmlItems []htmlItem // its items of format html, which Render checks the page drawn for
	marks     *htmlMarks // its items of format html marked, while checkDrawnHTML draws it; nil else
}

// shared are the keys of a data file floe reads itself, in every template; the rest are the schema's.
var shared = []string{"lang", "template", "theme", "title"}

// Read reads the data file src, checks it against its template's schema, then its checks, and
// computes the values of its computed.yaml; file names it in errors. A mistake is a *usererr.Error,
// and those of the schema, of the checks or of the computing are all reported at once, as a
// usererr.List.
//
// theme, when not "", is drawn with instead of the one the data names: the data's theme: is then not
// used, and a theme that does not exist is refused as a *usererr.Error, as is a template that needs
// another floe. A relative file is under cwd.
func (l *Library) Read(file string, src []byte, theme, cwd string) (*Data, error) {
	d, _, err := l.ReadSources(file, src, theme, cwd)
	return d, err
}

// Sources are the files a read of a data file read besides it, for a link to be drawn again when one
// changes, whether the read found mistakes or not.
type Sources struct {
	Templates string   // the directory of templates of one's own, where one may come; "" for none
	Own       []string // the directories of parts and themes of one's own read, whose files the data may be drawn with
	Template  string   // the directory of the template of one's own the data names; "" for none
	Images    []string // the images it names that a file can make right, read or not, as written: under ImagesDir
	ImagesDir string   // the directory of the data file on disk, its images under it
	Icons     []string // the index files of the icon sets, when it has a diagram, fetched or not
}

// ReadSources reads the data file src as Read does, and returns its Sources too.
func (l *Library) ReadSources(file string, src []byte, theme, cwd string) (*Data, Sources, error) {
	dir := filepath.Dir(file)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	d, sources, err := l.readWith(file, src, diskImages(dir), theme)
	sources.ImagesDir = dir
	return d, sources, err
}

// errNoImages is a ReadFS without an fs.FS to read the images from: a mistake of its caller.
var errNoImages = errors.New("tmpl.ReadFS: no fs.FS to read the images from")

// ReadFS reads the data file src as Read does, its images read from images, the directory of the data
// file, which its mistakes name dir. images must list its directories (fs.ReadDir), as os.DirFS and
// fstest.MapFS do: each name on an image's path is looked up in its directory, case and all. It returns
// the images the data names that a file can make right, read or not, as it writes them (Sources.Images):
// nothing of it is on disk to watch.
func (l *Library) ReadFS(file string, src []byte, images fs.FS, dir, theme string) (*Data, []string, error) {
	if images == nil {
		return nil, nil, errNoImages
	}
	d, sources, err := l.readWith(file, src, fsImages(images, dir), theme)
	return d, sources.Images, err
}

func (l *Library) readWith(file string, src []byte, images imageDir, theme string) (*Data, Sources, error) {
	sources := Sources{Templates: l.dir, Own: nil, Template: "", Images: nil, ImagesDir: "", Icons: nil}
	// Where one may come, when none has been read.
	for _, dir := range []string{l.mine.parts, l.mine.themes} {
		if dir != "" {
			sources.Own = append(sources.Own, dir)
		}
	}
	h, err := start(file, src, theme)
	if err != nil {
		return nil, sources, err
	}
	d, problems := h.d, h.problems
	fail := func(p yamlpos.Pos, key string, args ...any) {
		problems = append(problems, yamlpos.New(file, p, key, args...))
	}
	// Without its template, the data has no schema to be checked against.
	var t *Template
	if d.Template != "" {
		if t, err = l.Find(d.Template); err != nil {
			return nil, sources, err
		}
	}
	if t == nil {
		var names []string
		if names, err = l.Names(); err != nil {
			return nil, sources, err
		}
		switch {
		case h.unnamed:
			fail(h.where["template"], "tmpl_no_template", strings.Join(names, ", "))
		case d.Template != "":
			fail(h.where["template"], "tmpl_unknown", d.Template, strings.Join(names, ", "))
		}
		return nil, sources, problems.Sorted()
	}
	sources.Template = t.Dir
	// The floe a template needs comes before its mistakes, which another floe may not make.
	if err = t.checkVersion(); err != nil {
		return nil, sources, err
	}
	err = t.check(h, images, theme, problems)
	sources.Images, sources.Icons = d.tried, d.icons
	if t.set != nil && t.set.dirs != nil {
		sources.Own = t.set.dirs
	}
	if err != nil {
		return nil, sources, err
	}
	return d, sources, nil
}

// Images returns the images d shows as read, by the slash-separated path written, under the directory
// of its data file. They are decoded from the data URIs the drawing takes, so that a drawing does not
// hold each image twice for a publish to keep them.
func (d *Data) Images() (map[string][]byte, error) {
	images := make(map[string][]byte, len(d.images))
	for name, uri := range d.images {
		_, encoded, _ := strings.Cut(string(uri), ";base64,")
		b, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode the image %s: %w", name, err)
		}
		images[name] = b
	}
	return images, nil
}

// TitleOf returns the title: of the data file src, or "" when it names none or cannot be read.
func TitleOf(src []byte) string {
	h, err := start("", src, "")
	if err != nil {
		return ""
	}
	return h.d.Title
}

// head is a data file read up to its template.
type head struct {
	d    *Data // its values, the keys every template shares moved to d
	root ast.Node
	// where each key every template shares is written, the top of the file for one left out
	where    map[string]yamlpos.Pos
	problems usererr.List // the mistakes in those keys
	unnamed  bool         // template: is left out or empty, a mistake left to the caller to word
}

// start reads the data file src, file in its mistakes, up to its template, leaving out the data's
// theme when given replaces it. It fails with yamlpos.ErrEmpty for a file without a document.
func start(file string, src []byte, given string) (*head, error) {
	root, err := parse(file, src)
	if errors.Is(err, yamlpos.ErrEmpty) {
		// As an empty mapping, which is told every key it misses at its top.
		root, err = parse(file, []byte("{}"))
	}
	if err != nil {
		return nil, err
	}
	top, err := decode(file, root)
	if err != nil {
		return nil, err
	}
	d := &Data{
		File: file, Template: "", Theme: DefaultTheme, Title: "", Lang: "", Values: top, Calc: nil,
		tmpl: nil, shape: nil, images: nil, parts: nil, diagrams: false, tried: nil, icons: nil, htmlItems: nil, marks: nil,
	}
	h := &head{d: d, root: root, where: map[string]yamlpos.Pos{}, problems: nil, unnamed: false}
	h.readShared(given)
	return h, nil
}

// check checks h.d, whose images are under images, against t, which then draws it: with the theme
// given, else the data's. problems are those found before in the keys every template shares.
func (t *Template) check(h *head, images imageDir, given string, problems usererr.List) error {
	d := h.d
	d.tmpl = t
	set, err := t.parts()
	if err != nil {
		return err
	}
	more, err := pickTheme(set, d, given, h.where["theme"])
	if err != nil {
		return err
	}
	return checkValues(t, d, h.root, images, append(problems, more...))
}

// pickTheme sets d.Theme to given when not "", which must be one of the themes of set, else leaves the
// data's, reported at pos, where the data names it, when it is not one; one of one's own that cannot be
// used is told why.
func pickTheme(set *partSet, d *Data, given string, pos yamlpos.Pos) (usererr.List, error) {
	name := cmp.Or(given, d.Theme)
	names := set.themeNames()
	var why *usererr.Error
	switch {
	case !slices.Contains(names, name):
		why = usererr.New("tmpl_unknown_theme", name, strings.Join(names, ", "))
	case set.brokenThemes[name] != nil:
		why = set.brokenTheme(name)
	}
	switch {
	case why != nil && given != "":
		return nil, why
	case why != nil:
		return usererr.List{yamlpos.Place(d.File, pos, why)}, nil
	}
	d.Theme = name
	return nil, nil
}

// checkValues checks d against its template's schema, reads its images under images, checks it against
// its checks and computes its values, reporting the schema's mistakes with problems, those found before
// in the keys every template shares.
func checkValues(t *Template, d *Data, root ast.Node, images imageDir, problems usererr.List) error {
	// The template's own mistakes first: data checked against a schema floe refuses means nothing.
	s, doc, err := t.checkedShape()
	if err != nil {
		return err
	}
	set, err := t.parts()
	if err != nil {
		return err
	}
	err = validate(set, doc, d, root)
	var more usererr.List
	switch {
	case errors.As(err, &more):
		problems = append(problems, more...)
	case err != nil:
		return err
	}
	if problems != nil {
		return problems.Sorted()
	}
	d.shape = s
	whole := s.drawnWhole(d.Values)
	d.htmlItems = htmlItemsOf(whole, root)
	err = checkItems(checkImages(whole, d, root, images), checkHTML(set, d), checkDiagrams(whole, d, root, t.icons),
		checkScripts(whole, d, root), checkParts(whole, d, root))
	if err != nil {
		return err
	}
	// Each stage runs on data that passed the one before: broken data would only give the expressions
	// mistakes that mean nothing.
	if err = checkRelations(t, d, root); err != nil {
		return err
	}
	return checkComputed(t, d, root, s)
}

// checkHTML refuses the items of d of format html that hold a <style> or a <link>, at their lines and
// columns: floe puts a page's CSS together, in layers, and a style of the data would win over them all. It
// refuses those that hold a </head>, which would stand in for page.html's, before which floe puts the
// page's styles, and those that hold a <base> or a <meta>, which change the page without changing its
// tree (baseOrMeta). It refuses those that name a part of set that the page would refuse too.
func checkHTML(set *partSet, d *Data) error {
	var problems usererr.List
	for _, it := range d.htmlItems {
		// Wherever it is, a comment's text too.
		if styleOrLink.MatchString(it.text) {
			problems = append(problems, itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_style")))
		}
		if headEnd.MatchString(it.text) {
			problems = append(problems, itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_head")))
		}
		if baseOrMeta.MatchString(it.text) {
			problems = append(problems, itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_base")))
		}
		if bad := set.misnamedIn(namedIn(it.text, tagsIn(it.text, false))); bad != nil {
			problems = append(problems, itemError(d.File, it.pos, it.path, bad))
		}
	}
	return problems.Sorted().Err()
}

// readShared moves the keys every template shares from h.d.Values to h.d, and notes where each is
// written (the top of the file for one left out), the mistakes in them, and whether template: names
// none. A theme given over the data's leaves the data's theme: unread.
func (h *head) readShared(given string) {
	d := h.d
	to := map[string]*string{"lang": &d.Lang, "template": &d.Template, "theme": &d.Theme, "title": &d.Title}
	h.unnamed = true
	for _, key := range shared {
		h.where[key] = yamlpos.Of(h.root)
		v, ok := d.Values[key]
		if !ok {
			// The page's <html lang> is the data's own, so that every way of drawing it gives the same page.
			if key == "lang" {
				h.problems = append(h.problems, yamlpos.New(d.File, h.where[key], "tmpl_no_lang"))
			}
			continue
		}
		k, n, _ := yamlpos.Find(h.root, []string{key})
		h.where[key] = yamlpos.OfValue(k, n)
		delete(d.Values, key)
		if key == "theme" && given != "" {
			continue
		}
		s, text := v.(string)
		if key == "template" {
			// One that is not text is reported so.
			h.unnamed = text && s == ""
		}
		switch {
		case key == "lang" && text && !i18n.IsTag(s):
			h.problems = append(h.problems, yamlpos.New(d.File, h.where[key], "tmpl_lang_invalid", s))
		case text:
			*to[key] = s
		default:
			h.problems = append(h.problems, yamlpos.New(d.File, h.where[key], "tmpl_not_text", key))
		}
	}
	// A title of spaces names none, as an empty --title does.
	d.Title = strings.TrimSpace(d.Title)
}

// parse reads src as one YAML document of keys and values, without anchors, aliases or tags, so
// that every value is written where its position points, and without .inf or .nan, which JSON has no
// number for and which would make a comparison of the checks silently hold or fail.
func parse(file string, src []byte) (ast.Node, error) {
	body, err := yamlpos.Parse(file, src, "tmpl_documents")
	if err != nil {
		return nil, err //nolint:wrapcheck // a mistake at its line and column, or ErrEmpty
	}
	if err = plain(file, body); err != nil {
		return nil, err
	}
	if n := first(body, nonFinite); n != nil {
		return nil, yamlpos.New(file, yamlpos.Of(n), "tmpl_not_finite", n.GetToken().Value)
	}
	if _, ok := body.(*ast.MappingNode); !ok {
		return nil, yamlpos.New(file, yamlpos.Of(body), "tmpl_not_mapping")
	}
	return body, nil
}

// Render draws d, as read by Library.Read, as a page titled title in its lang:, with the template the
// data was checked against.
func (d *Data) Render(title string) ([]byte, error) {
	t := d.tmpl
	w, err := t.words()
	if err != nil {
		return nil, err
	}
	page, src, err := t.page(w, d.Lang)
	if err != nil {
		return nil, err
	}
	out, err := d.execute(page, title)
	if err != nil {
		return nil, err
	}
	own, err := t.ownCSS("page.css")
	if err != nil {
		return nil, err
	}
	set, err := t.parts()
	if err != nil {
		return nil, err
	}
	html := string(out)
	found := tagsIn(html, false)
	style, err := set.pageStyle(html, found, own, d.Theme)
	if err != nil {
		return nil, err
	}
	styled, ok := withStyles(out, found, style)
	if !ok {
		// An item's open <script> may hide the </head>: its mistake, not the template's.
		k := -1
		if d.htmlItems != nil {
			if k, err = d.headHider(page, title); err != nil {
				return nil, err
			}
		}
		if k >= 0 {
			it := d.htmlItems[k]
			return nil, itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_page"))
		}
		return nil, headMistake(t.file("page.html"), src)
	}
	// The data's HTML after the template's mistakes, which would be told as the data's.
	if d.htmlItems != nil {
		if err = d.checkDrawnHTML(page, title, out); err != nil {
			return nil, err
		}
	}
	return d.withDiagrams(styled)
}

// execute draws d with page, the template's page.html, as a page titled title, without its styles.
func (d *Data) execute(page *template.Template, title string) ([]byte, error) {
	var b bytes.Buffer
	err := page.Execute(&b, map[string]any{
		"Title": title,
		"Lang":  d.Lang,
		"Data":  d.shape.prepare(d.Values, nil, d),
		"Calc":  d.Calc,
	})
	// Read checked the data against the schema, so this is the template's own mistake: one a function of
	// page.html places itself (an image of its assets/ that is not right, a word its words.yaml has not),
	// or page.html naming an item its schema does not declare, or using one as another type.
	var bad *usererr.Error
	switch {
	case errors.As(err, &bad):
		return nil, bad
	case err != nil:
		return nil, usererr.New("tmpl_render", d.File, d.Template, err.Error())
	}
	return b.Bytes(), nil
}

// withDiagrams returns out, a page of d, with the script of its diagrams after it, once at the end, as the
// reload script is: page.html picks which diagrams it draws, and where.
func (d *Data) withDiagrams(out []byte) ([]byte, error) {
	if !d.diagrams {
		return out, nil
	}
	script, err := diagram.Script()
	if err != nil {
		return nil, err //nolint:wrapcheck // already says what failed
	}
	return append(out, "\n"+script+"\n"...), nil
}

// headMistake is the mistake of page.html, src, shown as file, whose drawing holds no </head>: it holds
// none, or an action of it leaves its own out. The data's HTML holds none, which checkHTML refuses.
func headMistake(file string, src []byte) *usererr.Error {
	if tagsOf(tagsIn(string(src), true)).head >= 0 {
		return usererr.NewAt(file, 0, 0, "tmpl_page_head_action")
	}
	return usererr.NewAt(file, 0, 0, "tmpl_page_head")
}

// page reads and parses the page.html of t, which holds no <style>: floe puts a page's CSS together, its
// words w, t's words.yaml, in the language tag lang, else en. It returns the source too. Without w (nil,
// as when words.yaml has a mistake told beside), a word page.html names is not checked.
func (t *Template) page(w words, lang string) (*template.Template, []byte, error) {
	src, err := t.readFile("page.html")
	if err != nil {
		return nil, nil, err
	}
	if at := tagsOf(tagsIn(string(src), true)).style; at >= 0 {
		return nil, nil, usererr.NewAt(t.file("page.html"), lineAt(src, at), 0, "tmpl_page_style")
	}
	if at := base.FindIndex(src); at != nil {
		return nil, nil, usererr.NewAt(t.file("page.html"), lineAt(src, at[0]), 0, "tmpl_page_base")
	}
	// missingkey=error stops a misspelt name in page.html instead of leaving a blank.
	page, err := template.New("page.html").Option("missingkey=error").Funcs(funcs).
		Funcs(template.FuncMap{"asset": t.assets(), "word": w.word(t.file("words.yaml"), w.lang(lang))}).
		Parse(string(src))
	if err != nil {
		return nil, nil, fmt.Errorf("parse the template %s: %w", t.Name, err)
	}
	if w != nil {
		if err = w.checkWords(page, t.file("page.html")); err != nil {
			return nil, nil, err
		}
	}
	return page, src, nil
}

// words reads the words.yaml of t, as readWords does.
func (t *Template) words() (words, error) {
	src, _, err := t.readOptional("words.yaml")
	if err != nil {
		return nil, err
	}
	return readWords(t.file("words.yaml"), src)
}

var funcs = template.FuncMap{
	"bytes":    formatBytes,
	"num":      formatNum,
	"markdown": markdownHTML,
	// markdownLine writes a line of Markdown, such as a table cell, as HTML without a paragraph around it.
	"markdownLine": func(text string) (template.HTML, error) {
		// The text is made one paragraph of unbroken lines: a blank line would split it, an indented
		// line would open no block, as markdownLine reads no indented code, and two spaces at the end
		// of a line would break it.
		var lines []string
		for line := range strings.Lines(text) {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		html, err := convert(markdownLine, "markdownLine", strings.Join(lines, "\n"))
		if err != nil {
			return "", err
		}
		html = strings.TrimSuffix(strings.TrimPrefix(html, "<p>"), "</p>\n")
		return template.HTML(html), nil //nolint:gosec // G203: goldmark leaves raw HTML out
	},
}

// markdownHTML writes Markdown text as HTML, a table in the table part's look. Raw HTML in it is left
// out: a data item takes HTML only when its schema says format: html.
func markdownHTML(text string) (template.HTML, error) {
	html, err := convert(markdown, "markdown", text)
	if err != nil {
		return "", err
	}
	// With raw HTML left out, every <table> in the output is one goldmark drew.
	html = strings.ReplaceAll(html, "<table>", `<div data-floe-part="table"><table>`)
	html = strings.ReplaceAll(html, "</table>", "</table></div>")
	return template.HTML(html), nil //nolint:gosec // G203: goldmark leaves raw HTML out
}

// convert writes text as HTML with md, the function of page.html called name.
func convert(md goldmark.Markdown, name, text string) (string, error) {
	var b bytes.Buffer
	if err := md.Convert([]byte(text), &b); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return b.String(), nil
}

// checkedShape reads the schema of t into its shape, and returns it as jsonschema reads it too. It refuses
// the first of the schema's mistakes floe template check tells (partSet.mistakes), and a key floe reads
// itself (refuseShared).
func (t *Template) checkedShape() (*shape, any, error) {
	doc, err := t.schemaDoc()
	if err != nil {
		return nil, nil, err
	}
	s, notes, err := t.readShape(doc)
	if err != nil {
		return nil, nil, err
	}
	set, err := t.parts()
	if err != nil {
		return nil, nil, err
	}
	if mistakes := set.mistakes(notes); mistakes != nil {
		return nil, nil, mistakes[0]
	}
	if err = t.refuseShared(s); err != nil {
		return nil, nil, err
	}
	return s, doc, nil
}

// schemaDoc reads the schema.json of t as jsonschema reads it.
func (t *Template) schemaDoc() (any, error) {
	src, err := t.readFile("schema.json")
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", t.Name, err)
	}
	if bad := checkDraft(doc); bad != nil {
		return nil, bad.Placed(t.file("schema.json"), 0, 0, "")
	}
	return doc, nil
}

// readShape reads doc, the schema of t, into its shape, bound to floe's parts, and returns with it what
// floe refuses of the schema, for the caller to refuse (partSet.mistakes).
func (t *Template) readShape(doc any) (*shape, *schemaNotes, error) {
	s, notes, err := readShape(doc, schemaURL(t.Name), t.file("schema.json"), "")
	if err != nil {
		return nil, nil, fmt.Errorf("read the schema of %s: %w", t.Name, err)
	}
	set, err := t.parts()
	if err != nil {
		return nil, nil, err
	}
	if name, ok := set.rootPart(s); ok {
		return nil, nil, usererr.NewAt(t.file("schema.json"), 0, 0, "tmpl_schema_root_part", name)
	}
	s.bind(set)
	return s, notes, nil
}

// refuseShared refuses a schema that declares, or requires, a key floe reads from every data file itself:
// floe takes it out of the data before the schema sees it, so the item would always be empty, and a
// required one always missing.
// LIMIT: the top of the schema, or what its root $ref names where it declares no items of its own (inside),
// not allOf; look there if a schema comes to put its items so.
func (t *Template) refuseShared(s *shape) error {
	in := s.inside()
	var named []string
	for _, key := range shared {
		if _, ok := in.keys[key]; ok || slices.Contains(in.required, key) {
			named = append(named, key)
		}
	}
	if named != nil {
		return usererr.NewAt(t.file("schema.json"), 0, 0, "tmpl_schema_shared", andList(named), andList(shared))
	}
	return nil
}

// prepare readies v, at the path at, for page.html: an item declared but not given becomes nil, so
// that page.html can test it under missingkey=error, an item a part draws becomes its HTML as d drew it
// (a diagram its SVG, a script its element and the call of its function), an item of format html HTML
// that goes in as it is, one of format markdown its HTML, and one of format image the data URI of its
// image. A part readies its own item too, the parts it holds drawn by d: its schema holds no format of
// floe's, which readPart refuses.
func (s *shape) prepare(v any, at []string, d *Data) any {
	if s == nil {
		return v
	}
	if drawn, whole := s.drawn(v, at, d); whole {
		return drawn
	}
	in := s.inside()
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v)+len(in.keys))
		for k, item := range v {
			out[k] = in.keys[k].prepare(item, append(at[:len(at):len(at)], k), d)
		}
		for k := range in.keys {
			if _, ok := out[k]; !ok {
				out[k] = nil
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = in.each.prepare(item, append(at[:len(at):len(at)], strconv.Itoa(i)), d)
		}
		return out
	}
	return v
}

// drawn returns v, at the path at, as d drew it whole, by a part or as an item of a format of floe's,
// and false when it is not drawn whole.
func (s *shape) drawn(v any, at []string, d *Data) (any, bool) {
	text, isText := v.(string)
	switch {
	case s.part != nil:
		return d.parts[pathKey(at)], true
	case s.format == "markdown" && isText:
		return d.markdownHTML(text), true
	case s.format == "html" && isText:
		if d != nil && d.marks != nil {
			text = d.marks.marked(text, at)
		}
		return template.HTML(text), true //nolint:gosec // G203: the schema declares this item HTML
	case s.format == "image" && isText:
		return d.images[text], true
	case slices.Contains(formats, s.format):
		return v, true
	}
	return nil, false
}

// itemAt is an item of a data file, and the path of keys to it.
type itemAt struct {
	value any
	at    []string
}

// pathKey keys an item by its path of keys.
func pathKey(at []string) string {
	return strings.Join(at, "\x00")
}

// itemMistake is the mistake e in the item at the path at under root, of the data file file: at its
// value, or at its key with onKey.
func itemMistake(file string, root ast.Node, at []string, onKey bool, e *usererr.Error) *usererr.Error {
	pos, path := itemPos(root, at, onKey)
	return itemError(file, pos, path, e)
}

// itemPos returns where the item at the path at is written under root, its value or, onKey, its key, and
// its path.
func itemPos(root ast.Node, at []string, onKey bool) (yamlpos.Pos, string) {
	key, value, path := yamlpos.Find(root, at)
	if onKey {
		return yamlpos.Of(key), path
	}
	return yamlpos.OfValue(key, value), path
}

// itemError is the mistake e in the item at path, written at pos in the data file file.
// LIMIT: the path stays in the words, not in At: the top level is worded in the reader's language. Give At a
// worded type if a reader comes to need the path apart, such as mistakes sent as a list with their places.
func itemError(file string, pos yamlpos.Pos, path string, e *usererr.Error) *usererr.Error {
	return yamlpos.Place(file, pos, usererr.New("tmpl_item", path, e))
}

// decode reads the values under root. goccy/go-yaml reads a whole number above 0 as uint64, which
// expr negates by wrapping around (-3 / 2 is 9.2e18), so one that fits becomes an int.
func decode(file string, root ast.Node) (map[string]any, error) {
	var top map[string]any
	if err := yaml.NodeToValue(root, &top); err != nil {
		return nil, yamlpos.Syntax(file, err)
	}
	ints(top)
	return top, nil
}

// ints turns the uint64 in v that fit into int, in place, and returns v.
func ints(v any) any {
	switch v := v.(type) {
	case uint64:
		if v <= math.MaxInt64 {
			return int(v)
		}
	case map[string]any:
		for k, item := range v {
			v[k] = ints(item)
		}
	case []any:
		for i, item := range v {
			v[i] = ints(item)
		}
	}
	return v
}

// first returns the first node under n that pick picks, or nil.
func first(n ast.Node, pick func(ast.Node) ast.Node) ast.Node {
	f := &finder{pick: pick, found: nil}
	ast.Walk(f, n)
	return f.found
}

type finder struct {
	pick  func(ast.Node) ast.Node
	found ast.Node
}

func (f *finder) Visit(n ast.Node) ast.Visitor {
	if f.found != nil {
		return nil
	}
	if f.found = f.pick(n); f.found != nil {
		return nil
	}
	return f
}

// nonTextKey picks the key of n that is not text. A key is a name: 0x10, ~ or 1.0 would be read as
// another name than the one written, and not found where it was written.
func nonTextKey(n ast.Node) ast.Node {
	v, ok := n.(*ast.MappingValueNode)
	if !ok {
		return nil
	}
	key := yamlpos.KeyOf(v)
	if _, text := key.(*ast.StringNode); text {
		return nil
	}
	return key
}

// plain refuses in body, the file's, what would hide where a value is written (an anchor, alias, merge
// key or tag) or read a key as another name than the one written.
func plain(file string, body ast.Node) error {
	if a := yamlpos.FirstAnchor(body); a != nil {
		return yamlpos.New(file, yamlpos.Of(a), "yaml_no_anchors")
	}
	if k := first(body, nonTextKey); k != nil {
		return yamlpos.New(file, yamlpos.Of(k), "tmpl_key_not_text", k.GetToken().Value)
	}
	return nil
}

// nonFinite picks .inf and .nan.
func nonFinite(n ast.Node) ast.Node {
	switch n.(type) {
	case *ast.InfinityNode, *ast.NanNode:
		return n
	}
	return nil
}
