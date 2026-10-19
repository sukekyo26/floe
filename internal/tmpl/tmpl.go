// Package tmpl draws pages from data files: a YAML file names a template, and the template's page.html
// lays its data out. The templates are built in (report). A page has two themes: its template's, a CSS
// file of the template for the page, and its parts', a CSS file of floe's for the parts.
package tmpl

import (
	"bytes"
	"cmp"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// templateFiles holds a directory per built-in template: page.html, schema.json, example.yaml and themes/.
//
//go:embed templates
var templateFiles embed.FS

const (
	// DefaultTheme is the theme of a page, and of its parts, whose data names none.
	DefaultTheme = "default"
)

// Themes are the themes to draw a page with instead of those its data names (floe publish --theme and
// --parts-theme), each "" for the data's.
type Themes struct {
	Template string
	Parts    string
}

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
	Theme    string
	// PartsTheme is the theme of floe's parts it is drawn with.
	PartsTheme string
	Title      string // "" when the data names none
	Lang       string // the language of the page, a BCP 47 tag
	Values     map[string]any
	Calc       map[string]any // the values of the template's computed.yaml, by name; nil without one

	tmpl   *Template               // the template Read checked the data against, which draws it
	schema *schema                 // the template's schema as Read read it, for drawing as checked
	images map[string]template.URL // its images as data URIs, by the path written
	// its items drawn by floe's parts, by the path of keys to each (pathKey); nil without one
	parts    map[string]template.HTML
	diagrams bool     // whether it has a diagram, for the page to get their style and zoom
	tried    []string // the files of the images it names, read or not
	icons    []string // the index files of the icon sets its diagrams may show, when it has one
}

// shared are the keys of a data file floe reads itself, in every template; the rest are the schema's.
var shared = []string{"lang", "parts-theme", "template", "theme", "title"}

// Read reads the data file src, checks it against its template's schema, then its checks, and
// computes the values of its computed.yaml; file names it in errors. A mistake is a *yamlpos.Error,
// and those of the schema, of the checks or of the computing are all reported at once as
// yamlpos.Problems.
//
// A theme of themes, when not "", is drawn with instead of the one the data names: the data's theme:
// or parts-theme: is then not used, and a theme that does not exist is refused as a *usererr.Error, as
// is a template that needs another floe. A relative file is under cwd.
func (l *Library) Read(file string, src []byte, themes Themes, cwd string) (*Data, error) {
	d, _, err := l.ReadSources(file, src, themes, cwd)
	return d, err
}

// Sources are the files a read of a data file read besides it, for a link to be drawn again when one
// changes, whether the read found mistakes or not.
type Sources struct {
	Templates string   // the directory of templates of one's own, where one may come; "" for none
	Template  string   // the directory of the template of one's own the data names; "" for none
	Images    []string // the images the data names, read or not
	Icons     []string // the index files of the icon sets, when it has a diagram, fetched or not
}

// ReadSources reads the data file src as Read does, and returns its Sources too.
func (l *Library) ReadSources(file string, src []byte, themes Themes, cwd string) (*Data, Sources, error) {
	sources := Sources{Templates: l.dir, Template: "", Images: nil, Icons: nil}
	dir := filepath.Dir(file)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	h, err := start(file, src, themes)
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
	err = t.check(h, dir, themes, problems)
	sources.Images, sources.Icons = d.tried, d.icons
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
	h, err := start("", src, Themes{Template: "", Parts: ""})
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
	problems yamlpos.Problems // the mistakes in those keys
	unnamed  bool             // template: is left out or empty, a mistake left to the caller to word
}

// start reads the data file src, file in its mistakes, up to its template, leaving out the data's
// themes that given replaces. It fails with yamlpos.ErrEmpty for a file without a document.
func start(file string, src []byte, given Themes) (*head, error) {
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
		File: file, Template: "", Theme: DefaultTheme, PartsTheme: DefaultTheme, Title: "", Lang: "", Values: top, Calc: nil,
		tmpl: nil, schema: nil, images: nil, parts: nil, diagrams: false, tried: nil, icons: nil,
	}
	h := &head{d: d, root: root, where: map[string]yamlpos.Pos{}, problems: nil, unnamed: false}
	h.readShared(given)
	return h, nil
}

// check checks h.d, whose images are under dir, against t, which then draws it: with the themes given,
// else the data's. problems are those found before in the keys every template shares.
func (t *Template) check(h *head, dir string, given Themes, problems yamlpos.Problems) error {
	d := h.d
	d.tmpl = t
	themes, err := t.themes()
	if err != nil {
		return err
	}
	partsThemes, err := PartsThemes()
	if err != nil {
		return err
	}
	// A theme of one kind named as one of the other is told where it goes.
	unknown := misplaced(cmp.Or(given.Template, d.Theme), themes, partsThemes, "tmpl_unknown_theme", "tmpl_theme_of_parts")
	more, err := pickTheme(d, &d.Theme, given.Template, themes, h.where["theme"], unknown)
	if err != nil {
		return err
	}
	problems = append(problems, more...)
	unknown = misplaced(cmp.Or(given.Parts, d.PartsTheme), partsThemes, themes, "tmpl_unknown_parts_theme",
		"tmpl_parts_theme_of_template")
	more, err = pickTheme(d, &d.PartsTheme, given.Parts, partsThemes, h.where["parts-theme"], unknown)
	if err != nil {
		return err
	}
	problems = append(problems, more...)
	return checkValues(t, d, h.root, dir, problems)
}

// misplaced returns the message of the theme name that is not one of names: elsewhere when it is one of
// others, the themes of the other kind, else unknown.
func misplaced(name string, names, others []string, unknown, elsewhere string) string {
	if !slices.Contains(names, name) && slices.Contains(others, name) {
		return elsewhere
	}
	return unknown
}

// pickTheme sets *theme, a theme of d, to given when not "", which must be one of names, else leaves the
// data's, reported at pos, where the data names it, when it is not one of them. unknown is the message
// of a theme that is not.
func pickTheme(d *Data, theme *string, given string, names []string, pos yamlpos.Pos, unknown string) (
	yamlpos.Problems, error,
) {
	switch {
	case given != "" && !slices.Contains(names, given):
		return nil, usererr.New(unknown, given, d.Template, strings.Join(names, ", "))
	case given != "":
		*theme = given
	case !slices.Contains(names, *theme):
		return yamlpos.Problems{yamlpos.New(d.File, pos, unknown, *theme, d.Template, strings.Join(names, ", "))}, nil
	}
	return nil, nil
}

// checkValues checks d against its template's schema, reads its images under dir, checks it against
// its checks and computes its values, reporting the schema's mistakes with problems, those found before
// in the keys every template shares.
func checkValues(t *Template, d *Data, root ast.Node, dir string, problems yamlpos.Problems) error {
	src, err := t.readFile("schema.json")
	if err != nil {
		return err
	}
	// The template's own mistakes first: data checked against a schema floe refuses means nothing.
	s, err := t.decodeSchema(src)
	if err != nil {
		return err
	}
	if err = t.refuseShared(s); err != nil {
		return err
	}
	err = validate(src, d, root)
	var more yamlpos.Problems
	switch {
	case errors.As(err, &more):
		problems = append(problems, more...)
	case err != nil:
		return err
	}
	if problems != nil {
		return problems.Sorted()
	}
	d.schema = s
	err = checkItems(checkImages(s, d, root, dir), checkHTML(s, d, root), checkDiagrams(s, d, root, t.icons),
		checkScripts(s, d, root), checkParts(s, d, root))
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

// checkHTML refuses the items of d of format html, as its schema s declares them, that hold a <style> or
// a <link>, at their lines and columns under root: floe puts a page's CSS together, in layers, and a
// style of the data would win over them all.
func checkHTML(s *schema, d *Data, root ast.Node) error {
	var found []itemAt
	s.find("html", d.Values, nil, &found)
	var problems yamlpos.Problems
	for _, f := range found {
		html, ok := f.value.(string)
		if !ok {
			continue
		}
		at := tagsOf([]byte(html))
		if at.style >= 0 || at.link >= 0 {
			problems = append(problems, itemMistake(d.File, root, f.at, false, usererr.New("tmpl_html_style")))
		}
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// readShared moves the keys every template shares from h.d.Values to h.d, and notes where each is
// written (the top of the file for one left out), the mistakes in them, and whether template: names
// none. A theme given over the data's leaves the data's theme: or parts-theme: unread.
func (h *head) readShared(given Themes) {
	d := h.d
	to := map[string]*string{
		"lang": &d.Lang, "parts-theme": &d.PartsTheme, "template": &d.Template, "theme": &d.Theme, "title": &d.Title,
	}
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
		if key == "theme" && given.Template != "" || key == "parts-theme" && given.Parts != "" {
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
	page, err := t.page()
	if err != nil {
		return nil, err
	}
	own, err := t.css(d.Theme)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = page.Execute(&b, map[string]any{
		"Title": title,
		"Lang":  d.Lang,
		"Data":  d.schema.prepare(d.Values, nil, d),
		"Calc":  d.Calc,
	})
	// Read checked the data against the schema, so this is the template's own mistake: an image of its
	// assets/ that is not right, or page.html naming an item its schema does not declare, or using one
	// as another type.
	var bad *usererr.Error
	switch {
	case errors.As(err, &bad):
		return nil, t.assetMistake(bad)
	case err != nil:
		return nil, usererr.New("tmpl_render", d.File, d.Template, err.Error())
	}
	out := b.Bytes()
	style, err := pageStyle(out, own, d.PartsTheme, d.diagrams)
	if err != nil {
		return nil, err
	}
	out = withStyles(out, style)
	// Once at the end, as the reload script is: page.html picks which diagrams it draws, and where.
	if d.diagrams {
		script, err := diagram.Script()
		if err != nil {
			return nil, err //nolint:wrapcheck // already says what failed
		}
		out = append(out, "\n"+script+"\n"...)
	}
	return out, nil
}

// css returns the template's own CSS: its page.css, empty if it has none, then its theme.
func (t *Template) css(theme string) ([][]byte, error) {
	page, err := t.ownCSS("page.css")
	if err != nil {
		return nil, err
	}
	css, err := t.ownCSS(path.Join("themes", theme+".css"))
	if err != nil {
		return nil, err
	}
	return [][]byte{page, css}, nil
}

// page reads and parses the page.html of t, which holds no <style>: floe puts a page's CSS together.
func (t *Template) page() (*template.Template, error) {
	src, err := t.readFile("page.html")
	if err != nil {
		return nil, err
	}
	if at := tagsOf(src).style; at >= 0 {
		return nil, usererr.New("tmpl_page_style", t.file("page.html"), lineAt(src, at))
	}
	// missingkey=error stops a misspelt name in page.html instead of leaving a blank.
	page, err := template.New("page.html").Option("missingkey=error").Funcs(funcs).
		Funcs(template.FuncMap{"asset": t.assets()}).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse the template %s: %w", t.Name, err)
	}
	return page, nil
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
	html = strings.ReplaceAll(html, "<table>", `<div class="floe-table"><table>`)
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

// schemaOf reads the schema of t.
func schemaOf(t *Template) (*schema, error) {
	b, err := t.readFile("schema.json")
	if err != nil {
		return nil, err
	}
	s, err := t.decodeSchema(b)
	if err == nil {
		err = t.refuseShared(s)
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// decodeSchema reads b, the schema.json of t, with what it refers to among floe's parts, and refuses a
// ref into a part and a part floe draws in Go written as a format, which would be drawn as its text.
func (t *Template) decodeSchema(b []byte) (*schema, error) {
	s, err := t.readSchema(b)
	if err != nil {
		return nil, err
	}
	set, err := builtinParts()
	if err != nil {
		return nil, err
	}
	if _, into := s.first("#", func(x *schema) bool { _, into := set.pointsInto(x.Ref); return into }); into != nil {
		return nil, usererr.New("tmpl_item", t.file("schema.json"), unknownPart(into.Ref))
	}
	if ptr, as := s.first("#", func(x *schema) bool { return isGoPart(x.Format) }); as != nil {
		return nil, usererr.New("tmpl_format_part", t.file("schema.json"), ptr, as.Format, as.Format)
	}
	return s, nil
}

// readSchema reads b, the schema.json of t, with what it refers to among floe's parts, a ref into a
// part left for the caller to refuse.
func (t *Template) readSchema(b []byte) (*schema, error) {
	s, err := decodeSchema(b)
	if err != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", t.Name, err)
	}
	set, err := builtinParts()
	if err != nil {
		return nil, err
	}
	s.absolute(schemaURL(t.Name))
	s.resolve(set)
	return s, nil
}

// refuseShared refuses a schema that declares, or requires, a key floe reads from every data file itself:
// floe takes it out of the data before the schema sees it, so the item would always be empty, and a
// required one always missing.
// LIMIT: the top of the schema only, not through $ref or allOf; look there if a schema comes to put its
// items so.
func (t *Template) refuseShared(s *schema) error {
	var named []string
	for _, key := range shared {
		if _, ok := s.Properties[key]; ok || slices.Contains(s.Required, key) {
			named = append(named, key)
		}
	}
	if named != nil {
		return usererr.New("tmpl_schema_shared", t.file("schema.json"), andList(named), andList(shared))
	}
	return nil
}

var errAfterSchema = errors.New("text after the schema")

// decodeSchema reads the JSON Schema b, its numbers as written, so that an enum of 1000000 is not
// shown as 1e+06.
func decodeSchema(b []byte) (*schema, error) {
	var s schema
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&s); err != nil {
		return nil, err //nolint:wrapcheck // wrapped by schemaOf
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errAfterSchema
	}
	return &s, nil
}

// schema is the part of a template's JSON Schema that floe reads: the items it declares, their format
// or the part of floe's that draws them, and what floe template show tells of them.
type schema struct {
	Format     string             `json:"format"`
	Ref        string             `json:"$ref"`
	Properties map[string]*schema `json:"properties"`
	Items      *schema            `json:"items"`

	part *part // the part of floe's $ref names; nil for none

	// What floe template show tells of an item.
	Description string   `json:"description"`
	Type        any      `json:"type"` // a JSON type, or a list of them
	Enum        []any    `json:"enum"`
	Required    []string `json:"required"`
}

// prepare readies v, at the path at, for page.html: an item declared but not given becomes nil, so
// that page.html can test it under missingkey=error, an item a part draws becomes its HTML as d drew it
// (a diagram its SVG, a script its element and the call of its function), an item of format html HTML
// that goes in as it is, one of format markdown its HTML, and one of format image the data URI of its
// image. A part readies its own item with d nil: it holds no item of a part or a format.
func (s *schema) prepare(v any, at []string, d *Data) any {
	if s == nil {
		return v
	}
	if drawn, whole := s.drawn(v, at, d); whole {
		return drawn
	}
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v)+len(s.Properties))
		for k, item := range v {
			out[k] = s.Properties[k].prepare(item, append(at[:len(at):len(at)], k), d)
		}
		for k := range s.Properties {
			if _, ok := out[k]; !ok {
				out[k] = nil
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = s.Items.prepare(item, append(at[:len(at):len(at)], strconv.Itoa(i)), d)
		}
		return out
	}
	return v
}

// drawn returns v, at the path at, as d drew it whole, by a part or as an item of a format of floe's,
// and false when it is not drawn whole.
func (s *schema) drawn(v any, at []string, d *Data) (any, bool) {
	text, isText := v.(string)
	switch {
	case s.part != nil:
		return d.parts[pathKey(at)], true
	case s.Format == "markdown" && isText:
		html, _ := markdownHTML(text) //nolint:errcheck // goldmark fails only to write, and a buffer does not
		return html, true
	case s.Format == "html" && isText:
		return template.HTML(text), true //nolint:gosec // G203: the schema declares this item HTML
	case s.Format == "image" && isText:
		return d.images[text], true
	case slices.Contains(formats, s.Format):
		return v, true
	}
	return nil, false
}

// itemAt is an item of a data file, and the path of keys to it.
type itemAt struct {
	value any
	at    []string
}

// find adds to found the items v holds where s declares the format, v being at the path at.
func (s *schema) find(format string, v any, at []string, found *[]itemAt) {
	if s == nil {
		return
	}
	// A part, or an item of a format of floe's, is drawn whole: what it holds is not looked into.
	if s.part != nil {
		return
	}
	if slices.Contains(formats, s.Format) {
		if s.Format == format {
			*found = append(*found, itemAt{value: v, at: at})
		}
		return
	}
	switch v := v.(type) {
	case map[string]any:
		for k, item := range v {
			s.Properties[k].find(format, item, append(at[:len(at):len(at)], k), found)
		}
	case []any:
		for i, item := range v {
			s.Items.find(format, item, append(at[:len(at):len(at)], strconv.Itoa(i)), found)
		}
	}
}

// pathKey keys an item by its path of keys.
func pathKey(at []string) string {
	return strings.Join(at, "\x00")
}

// itemMistake is the mistake e in the item at the path at under root, of the data file file: at its
// value, or at its key with onKey.
func itemMistake(file string, root ast.Node, at []string, onKey bool, e *usererr.Error) *yamlpos.Error {
	key, value, path := yamlpos.Find(root, at)
	pos := yamlpos.OfValue(key, value)
	if onKey {
		pos = yamlpos.Of(key)
	}
	return &yamlpos.Error{File: file, Pos: pos, Err: usererr.New("tmpl_item", path, e)}
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
