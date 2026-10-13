// Package tmpl draws pages from data files: a YAML file names a template, and the template's page.html
// lays its data out. The templates are built in (report); a theme is a CSS file of a template.
package tmpl

import (
	"bytes"
	"embed"
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

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// builtin holds a directory per built-in template: page.html, schema.json, example.yaml and themes/.
//
//go:embed builtin
var builtin embed.FS

const (
	// DefaultTheme is the theme of a page whose data names none.
	DefaultTheme = "default"
	// DefaultLang is the language of a page unless the publisher names another.
	DefaultLang = string(i18n.English)
)

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// Data is a data file read: the keys every template shares, and the rest for the template's schema.
type Data struct {
	File     string
	Template string
	Theme    string
	Title    string // "" when the data names none
	Values   map[string]any
	Calc     map[string]any // the values of the template's computed.yaml, by name; nil without one

	tmpl   *Template               // the template Read checked the data against, which draws it
	schema *schema                 // the template's schema as Read read it, for drawing as checked
	images map[string]template.URL // its images as data URIs, by the path written
	// its diagrams drawn, by the path of keys to each (pathKey); nil without one
	diagrams map[string]template.HTML
	tried    []string // the files of the images it names, read or not
}

// shared are the keys of a data file floe reads itself, in every template; the rest are the schema's.
var shared = []string{"template", "theme", "title"}

// Read reads the data file src, checks it against its template's schema, then its checks, and
// computes the values of its computed.yaml; file names it in errors. A mistake is a *yamlpos.Error,
// and those of the schema, of the checks or of the computing are all reported at once as
// yamlpos.Problems.
//
// theme, when not "", is the theme to draw with instead of the one the data names (floe publish
// --theme): the data's theme: is then not used, and a theme the template does not have is refused as
// a *usererr.Error, as is a template that needs another floe. A relative file is under cwd.
func (l *Library) Read(file string, src []byte, theme, cwd string) (*Data, error) {
	d, _, err := l.ReadSources(file, src, theme, cwd)
	return d, err
}

// Sources are the files a read of a data file read besides it, for a link to be drawn again when one
// changes, whether the read found mistakes or not.
// LIMIT: not the icons of its diagrams: fetching them does not reload an open page, which shows them
// once reloaded by hand. Watch their index if that comes to be needed often.
type Sources struct {
	Templates string   // the directory of templates of one's own, where one may come; "" for none
	Template  string   // the directory of the template of one's own the data names; "" for none
	Images    []string // the images the data names, read or not
}

// ReadSources reads the data file src as Read does, and returns its Sources too.
func (l *Library) ReadSources(file string, src []byte, theme, cwd string) (*Data, Sources, error) {
	sources := Sources{Templates: l.dir, Template: "", Images: nil}
	dir := filepath.Dir(file)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	h, err := start(file, src, theme != "")
	if errors.Is(err, yamlpos.ErrEmpty) {
		var names []string
		if names, err = l.Names(); err != nil {
			return nil, sources, err
		}
		return nil, sources, yamlpos.New(file, yamlpos.Pos{Line: 1, Column: 1}, "tmpl_no_template", strings.Join(names, ", "))
	}
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
	err = t.check(d, h.root, dir, theme, h.where["theme"], problems)
	sources.Images = d.tried
	if err != nil {
		return nil, sources, err
	}
	return d, sources, nil
}

// TitleOf returns the title: of the data file src, or "" when it names none or cannot be read.
func TitleOf(src []byte) string {
	h, err := start("", src, false)
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

// start reads the data file src, file in its mistakes, up to its template. With themed, a theme is
// given over the data's. It fails with yamlpos.ErrEmpty for a file without a document.
func start(file string, src []byte, themed bool) (*head, error) {
	root, err := parse(file, src)
	if err != nil {
		return nil, err
	}
	top, err := decode(file, root)
	if err != nil {
		return nil, err
	}
	d := &Data{
		File: file, Template: "", Theme: DefaultTheme, Title: "", Values: top, Calc: nil, tmpl: nil, schema: nil,
		images: nil, diagrams: nil, tried: nil,
	}
	h := &head{d: d, root: root, where: map[string]yamlpos.Pos{}, problems: nil, unnamed: false}
	h.readShared(themed)
	return h, nil
}

// check checks d, whose root is root and whose images are under dir, against t, which then draws it:
// with the theme theme when given, else the data's, written at at. problems are those found before in
// the keys every template shares.
func (t *Template) check(d *Data, root ast.Node, dir, theme string, at yamlpos.Pos, problems yamlpos.Problems) error {
	d.tmpl = t
	more, err := pickTheme(t, d, theme, at)
	if err != nil {
		return err
	}
	return checkValues(t, d, root, dir, append(problems, more...))
}

// pickTheme sets the theme of d: theme when given, which t must have, else the data's, reported at
// pos, where the data names it, when t does not have it.
func pickTheme(t *Template, d *Data, theme string, pos yamlpos.Pos) (yamlpos.Problems, error) {
	themes, err := t.themes()
	if err != nil {
		return nil, err
	}
	switch {
	case theme != "" && !slices.Contains(themes, theme):
		return nil, usererr.New("tmpl_unknown_theme", theme, d.Template, strings.Join(themes, ", "))
	case theme != "":
		d.Theme = theme
	case !slices.Contains(themes, d.Theme):
		e := yamlpos.New(d.File, pos, "tmpl_unknown_theme", d.Theme, d.Template, strings.Join(themes, ", "))
		return yamlpos.Problems{e}, nil
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
	s, err := t.decodeSchema(src)
	if err != nil {
		return err
	}
	d.schema = s
	if err = checkItems(checkImages(s, d, root, dir), checkDiagrams(s, d, root, t.icons)); err != nil {
		return err
	}
	// Each stage runs on data that passed the one before: broken data would only give the expressions
	// mistakes that mean nothing.
	if err = checkRelations(t, d, root); err != nil {
		return err
	}
	return checkComputed(t, d, root, s)
}

// readShared moves the keys every template shares from h.d.Values to h.d, and notes where each is
// written (the top of the file for one left out), the mistakes in them, and whether template: names
// none. With themed, a theme is given over the data's, and its theme: is not used.
func (h *head) readShared(themed bool) {
	d := h.d
	to := map[string]*string{"template": &d.Template, "theme": &d.Theme, "title": &d.Title}
	h.unnamed = true
	for _, key := range shared {
		h.where[key] = yamlpos.Of(h.root)
		v, ok := d.Values[key]
		if !ok {
			continue
		}
		k, n, _ := yamlpos.Find(h.root, []string{key})
		h.where[key] = yamlpos.OfValue(k, n)
		delete(d.Values, key)
		if key == "theme" && themed {
			continue
		}
		s, text := v.(string)
		if key == "template" {
			// One that is not text is reported so.
			h.unnamed = text && s == ""
		}
		if text {
			*to[key] = s
		} else {
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

// Render draws d, as read by Library.Read, as a page titled title in lang (a BCP 47 tag; "" is
// DefaultLang), with the template the data was checked against.
func (d *Data) Render(title, lang string) ([]byte, error) {
	if lang == "" {
		lang = DefaultLang
	}
	t := d.tmpl
	page, err := t.page()
	if err != nil {
		return nil, err
	}
	css, err := t.readFile(path.Join("themes", d.Theme+".css"))
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = page.Execute(&b, map[string]any{
		"Title": title,
		"Lang":  lang,
		"Data":  d.schema.prepare(d.Values, nil, d),
		"Calc":  d.Calc,
		"Theme": template.CSS(css), //nolint:gosec // G203: the template's own theme, not data
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
	// Once at the end, as the reload script is: page.html picks which diagrams it draws, and where.
	if d.diagrams != nil {
		assets, err := diagram.Assets()
		if err != nil {
			return nil, err //nolint:wrapcheck // already says what failed
		}
		b.WriteString("\n" + assets + "\n")
	}
	return b.Bytes(), nil
}

// page reads and parses the page.html of t.
func (t *Template) page() (*template.Template, error) {
	src, err := t.readFile("page.html")
	if err != nil {
		return nil, err
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
	"bytes": formatBytes,
	"num":   formatNum,
	// markdown writes Markdown text as HTML, a table wrapped in <div class="table-wrap"> for a theme to
	// scroll. Raw HTML in it is left out: a data item takes HTML only when its schema says format: html.
	"markdown": func(text string) (template.HTML, error) {
		var b bytes.Buffer
		if err := markdown.Convert([]byte(text), &b); err != nil {
			return "", fmt.Errorf("markdown: %w", err)
		}
		// With raw HTML left out, every <table> in the output is one goldmark drew.
		html := strings.ReplaceAll(b.String(), "<table>", `<div class="table-wrap"><table>`)
		html = strings.ReplaceAll(html, "</table>", "</table></div>")
		return template.HTML(html), nil //nolint:gosec // G203: goldmark leaves raw HTML out
	},
}

// schemaOf reads the schema of t.
func schemaOf(t *Template) (*schema, error) {
	b, err := t.readFile("schema.json")
	if err != nil {
		return nil, err
	}
	return t.decodeSchema(b)
}

// decodeSchema reads b, the schema.json of t.
func (t *Template) decodeSchema(b []byte) (*schema, error) {
	s, err := decodeSchema(b)
	if err != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", t.Name, err)
	}
	return s, nil
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

// schema is the part of a template's JSON Schema that floe reads: the items it declares, their format,
// and what floe template show tells of them.
type schema struct {
	Format     string             `json:"format"`
	Properties map[string]*schema `json:"properties"`
	Items      *schema            `json:"items"`

	// What floe template show tells of an item.
	Description string   `json:"description"`
	Type        any      `json:"type"` // a JSON type, or a list of them
	Enum        []any    `json:"enum"`
	Required    []string `json:"required"`
}

// prepare readies v, at the path at, for page.html: an item declared but not given becomes nil, so
// that page.html can test it under missingkey=error, an item of format html becomes HTML that goes in
// as it is, one of format image the data URI of its image, and one of format diagram its SVG, both as
// d read them.
func (s *schema) prepare(v any, at []string, d *Data) any {
	if s == nil {
		return v
	}
	if s.Format == "diagram" {
		return d.diagrams[pathKey(at)]
	}
	switch v := v.(type) {
	case string:
		switch s.Format {
		case "html":
			return template.HTML(v) //nolint:gosec // G203: the schema declares this item HTML
		case "image":
			return d.images[v]
		}
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
	// An item of a format of floe's is drawn whole: what it holds is not looked into.
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
