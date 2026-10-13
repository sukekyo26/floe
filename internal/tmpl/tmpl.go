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
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
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

// Read reads the data file src; file names it in errors.
func Read(file string, src []byte) (*Data, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc, more any
	if err := dec.Decode(&doc); err != nil && !errors.Is(err, io.EOF) {
		return nil, usererr.New("tmpl_yaml", file, yaml.FormatError(err, false, false))
	}
	// A document after --- would otherwise be left out without a word.
	switch err := dec.Decode(&more); {
	case err == nil:
		return nil, usererr.New("tmpl_documents", file)
	case !errors.Is(err, io.EOF):
		return nil, usererr.New("tmpl_yaml", file, yaml.FormatError(err, false, false))
	}
	if doc == nil {
		return nil, usererr.New("tmpl_no_template", file, strings.Join(Names(), ", "))
	}
	top, ok := doc.(map[string]any)
	if !ok {
		return nil, usererr.New("tmpl_not_mapping", file)
	}
	d := &Data{File: file, Template: "", Theme: DefaultTheme, Title: "", Values: top}
	for _, k := range []struct {
		key string
		to  *string
	}{{"template", &d.Template}, {"theme", &d.Theme}, {"title", &d.Title}} {
		v, ok := top[k.key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return nil, usererr.New("tmpl_not_text", file, k.key)
		}
		*k.to = s
		delete(top, k.key)
	}
	// A title of spaces names none, as an empty --title does.
	d.Title = strings.TrimSpace(d.Title)
	if d.Template == "" {
		return nil, usererr.New("tmpl_no_template", file, strings.Join(Names(), ", "))
	}
	if !slices.Contains(Names(), d.Template) {
		return nil, usererr.New("tmpl_unknown", file, d.Template, strings.Join(Names(), ", "))
	}
	themes, err := themesOf(d.Template)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(themes, d.Theme) {
		return nil, usererr.New("tmpl_unknown_theme", file, d.Theme, d.Template, strings.Join(themes, ", "))
	}
	return d, nil
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
		// LIMIT: until the data is checked against the schema (phase 4b), data of the wrong shape
		// fails here, worded as Go's template error, and some data the schema refuses draws without a
		// word (a misspelt key is left out; a part of two kinds draws both).
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
