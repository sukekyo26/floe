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
	"io/fs"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

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
}

// shared are the keys of a data file floe reads itself, in every template; the rest are the schema's.
var shared = []string{"template", "theme", "title"}

// Read reads the data file src and checks it against its template's schema, then its checks; file
// names it in errors. A mistake is a *yamlpos.Error, and those of the schema or of the checks are all
// reported at once as yamlpos.Problems.
//
// theme, when not "", is the theme to draw with instead of the one the data names (floe publish
// --theme): the data's theme: is then not used, and a theme the template does not have is refused as
// a *usererr.Error.
func Read(file string, src []byte, theme string) (*Data, error) {
	root, err := parse(file, src)
	if err != nil {
		return nil, err
	}
	top, err := decode(file, root)
	if err != nil {
		return nil, err
	}
	d := &Data{File: file, Template: "", Theme: DefaultTheme, Title: "", Values: top}
	where, problems := readShared(d, root, theme != "")
	fail := func(p yamlpos.Pos, key string, args ...any) {
		problems = append(problems, yamlpos.New(file, p, key, args...))
	}
	// Without its template, the data has no schema to be checked against.
	switch {
	case d.Template == "" && problems == nil:
		fail(where["template"], "tmpl_no_template", strings.Join(Names(), ", "))
		return nil, problems.Sorted()
	case d.Template == "":
		return nil, problems.Sorted()
	case !slices.Contains(Names(), d.Template):
		fail(where["template"], "tmpl_unknown", d.Template, strings.Join(Names(), ", "))
		return nil, problems.Sorted()
	}
	themes, err := themesOf(d.Template)
	if err != nil {
		return nil, err
	}
	switch {
	case theme != "" && !slices.Contains(themes, theme):
		return nil, usererr.New("tmpl_unknown_theme", theme, d.Template, strings.Join(themes, ", "))
	case theme != "":
		d.Theme = theme
	case !slices.Contains(themes, d.Theme):
		fail(where["theme"], "tmpl_unknown_theme", d.Theme, d.Template, strings.Join(themes, ", "))
	}
	if err = checkValues(d, root, problems); err != nil {
		return nil, err
	}
	return d, nil
}

// checkValues checks d against its template's schema and then its checks, reporting the schema's
// mistakes with problems, those found before in the keys every template shares.
func checkValues(d *Data, root ast.Node, problems yamlpos.Problems) error {
	err := checkSchema(d, root)
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
	// Stage 3 runs on data that passed stage 2, its shared keys and its schema: broken data would only
	// give the checks' expressions mistakes that mean nothing.
	return checkRelations(d, root)
}

// readShared moves the keys every template shares from d.Values to d, and returns where each is
// written (the top of the file for one left out) and the mistakes in them. With themed, a theme is
// given over the data's, and its theme: is not used.
func readShared(d *Data, root ast.Node, themed bool) (map[string]yamlpos.Pos, yamlpos.Problems) {
	var problems yamlpos.Problems
	where := map[string]yamlpos.Pos{}
	to := map[string]*string{"template": &d.Template, "theme": &d.Theme, "title": &d.Title}
	for _, key := range shared {
		where[key] = yamlpos.Of(root)
		v, ok := d.Values[key]
		if !ok {
			continue
		}
		k, n, _ := yamlpos.Find(root, []string{key})
		where[key] = yamlpos.OfValue(k, n)
		delete(d.Values, key)
		if key == "theme" && themed {
			continue
		}
		if s, ok := v.(string); ok {
			*to[key] = s
		} else {
			problems = append(problems, yamlpos.New(d.File, where[key], "tmpl_not_text", key))
		}
	}
	// A title of spaces names none, as an empty --title does.
	d.Title = strings.TrimSpace(d.Title)
	return where, problems
}

// parse reads src as one YAML document of keys and values, without anchors, aliases or tags, so
// that every value is written where its position points, and without .inf or .nan, which JSON has no
// number for and which would make a comparison of the checks silently hold or fail.
func parse(file string, src []byte) (ast.Node, error) {
	body, err := yamlpos.Parse(file, src, "tmpl_documents")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return nil, yamlpos.New(file, yamlpos.Pos{Line: 1, Column: 1}, "tmpl_no_template", strings.Join(Names(), ", "))
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // a mistake at its line and column
	}
	if a := yamlpos.FirstAnchor(body); a != nil {
		return nil, yamlpos.New(file, yamlpos.Of(a), "yaml_no_anchors")
	}
	if k := first(body, nonTextKey); k != nil {
		return nil, yamlpos.New(file, yamlpos.Of(k), "tmpl_key_not_text", k.GetToken().Value)
	}
	if n := first(body, nonFinite); n != nil {
		return nil, yamlpos.New(file, yamlpos.Of(n), "tmpl_not_finite", n.GetToken().Value)
	}
	if _, ok := body.(*ast.MappingNode); !ok {
		return nil, yamlpos.New(file, yamlpos.Of(body), "tmpl_not_mapping")
	}
	return body, nil
}

// Names are the templates there are, by name.
func Names() []string {
	entries, err := builtin.ReadDir("builtin")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func themesOf(name string) ([]string, error) {
	entries, err := builtin.ReadDir(path.Join("builtin", name, "themes"))
	if err != nil {
		return nil, fmt.Errorf("read the themes of %s: %w", name, err)
	}
	themes := make([]string, 0, len(entries))
	for _, e := range entries {
		if t, ok := strings.CutSuffix(e.Name(), ".css"); ok {
			themes = append(themes, t)
		}
	}
	return themes, nil
}

// Render draws d as a page titled title in lang (a BCP 47 tag; "" is DefaultLang).
func (d *Data) Render(title, lang string) ([]byte, error) {
	if lang == "" {
		lang = DefaultLang
	}
	dir := path.Join("builtin", d.Template)
	src, err := fs.ReadFile(builtin, path.Join(dir, "page.html"))
	if err != nil {
		return nil, fmt.Errorf("read the template %s: %w", d.Template, err)
	}
	css, err := fs.ReadFile(builtin, path.Join(dir, "themes", d.Theme+".css"))
	if err != nil {
		return nil, fmt.Errorf("read the theme %s of %s: %w", d.Theme, d.Template, err)
	}
	var s schema
	if b, rerr := fs.ReadFile(builtin, path.Join(dir, "schema.json")); rerr != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", d.Template, rerr)
	} else if err = json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", d.Template, err)
	}
	// missingkey=error stops a misspelt name in page.html instead of leaving a blank.
	page, err := template.New("page.html").Option("missingkey=error").Funcs(funcs).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse the template %s: %w", d.Template, err)
	}
	var b bytes.Buffer
	err = page.Execute(&b, map[string]any{
		"Title": title,
		"Lang":  lang,
		"Data":  s.prepare(d.Values),
		"Theme": template.CSS(css), //nolint:gosec // G203: the template's own theme, not data
	})
	if err != nil {
		// Read checked the data against the schema, so this is the template's own mistake: page.html
		// naming an item its schema does not declare, or using one as another type.
		return nil, usererr.New("tmpl_render", d.File, d.Template, err.Error())
	}
	return b.Bytes(), nil
}

var funcs = template.FuncMap{
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

// schema is the part of a template's JSON Schema that drawing reads: the items it declares and their
// format.
type schema struct {
	Format     string             `json:"format"`
	Properties map[string]*schema `json:"properties"`
	Items      *schema            `json:"items"`
}

// prepare readies v for page.html: an item declared but not given becomes nil, so that page.html can
// test it under missingkey=error, and an item of format html becomes HTML that goes in as it is.
func (s *schema) prepare(v any) any {
	if s == nil {
		return v
	}
	switch v := v.(type) {
	case string:
		if s.Format == "html" {
			return template.HTML(v) //nolint:gosec // G203: the schema declares this item HTML
		}
	case map[string]any:
		out := make(map[string]any, len(v)+len(s.Properties))
		for k, item := range v {
			out[k] = s.Properties[k].prepare(item)
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
			out[i] = s.Items.prepare(item)
		}
		return out
	}
	return v
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
	key := ast.Node(v.Key)
	if explicit, ok := key.(*ast.MappingKeyNode); ok {
		key = explicit.Value
	}
	if _, text := key.(*ast.StringNode); text {
		return nil
	}
	return key
}

// nonFinite picks .inf and .nan.
func nonFinite(n ast.Node) ast.Node {
	switch n.(type) {
	case *ast.InfinityNode, *ast.NanNode:
		return n
	}
	return nil
}
