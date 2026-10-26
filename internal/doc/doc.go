// Package doc reads a data file: its values and where each is written, and the keys every template
// shares (lang, template, theme, title), which floe reads itself.
package doc

import (
	"errors"
	"math"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Data is a data file read: the keys every template shares, and the rest for the template's schema.
type Data struct {
	File                         string
	Lang, Template, Theme, Title string // "" for one left out
	Values                       map[string]any
	Root                         *ast.MappingNode // nil for an empty file
	// where each shared key is written: its value, its key when the value is left out, the top of the file
	// for one left out
	Where map[string]yamlpos.Pos
}

// shared are the keys of a data file floe reads itself, in every template; the rest are the schema's.
var shared = []string{"lang", "template", "theme", "title"}

// Read reads the data file src, file in its mistakes: yamlpos.Parse, and only when it refused nothing,
// goccy/go-yaml's reading of the values (its mistake for a top that is not keys and values), then the
// shared keys, each text, and lang: written and not empty or spaces. An empty file, or one of comments
// alone, reads as {}, its mistakes at 1:1, as it has no node to place them at. goccy/go-yaml reads a whole
// number above 0 as uint64, which expr negates by wrapping around (-3 / 2 is 9.2e18), so one that fits
// becomes an int: every layer sees the same values.
func Read(file string, src []byte) (*Data, error) {
	body, err := yamlpos.Parse(file, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
		body = nil
	case err != nil:
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, or floe's failure
	}
	values := map[string]any{}
	if body != nil {
		if err = yaml.NodeToValue(body, &values); err != nil {
			return nil, yamlpos.Library(file, err) //nolint:wrapcheck // goccy's mistake at its place
		}
		ints(values)
	}
	// goccy/go-yaml read the values into a map: the top is keys and values, or there is none.
	root, _ := body.(*ast.MappingNode) //nolint:errcheck // see above
	d := &Data{
		File: file, Lang: "", Template: "", Theme: "", Title: "", Values: values, Root: root,
		Where: map[string]yamlpos.Pos{},
	}
	return d, d.readShared()
}

// ints turns the uint64 in v that fit into int, in place, and returns v.
// LIMIT: a whole number above 9.2e18 stays uint64, which expr's - still wraps around (-a < 0 does not hold).
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

// readShared moves the keys every template shares from d.Values to d, and notes where each is written.
func (d *Data) readShared() error {
	top := yamlpos.Pos{Line: 1, Column: 1}
	pairs := map[string]*ast.MappingValueNode{}
	if d.Root != nil {
		top = yamlpos.Of(d.Root)
		for _, v := range d.Root.Values {
			pairs[yamlpos.KeyOf(v).GetToken().Value] = v
		}
	}
	to := map[string]*string{"lang": &d.Lang, "template": &d.Template, "theme": &d.Theme, "title": &d.Title}
	var found usererr.List
	for _, key := range shared {
		d.Where[key] = top
		if p := pairs[key]; p != nil {
			d.Where[key] = yamlpos.OfValue(yamlpos.KeyOf(p), p.Value)
		}
		v, ok := d.Values[key]
		delete(d.Values, key)
		s, text := v.(string)
		switch {
		case ok && !text:
			found = append(found, yamlpos.New(d.File, d.Where[key], "doc_not_text", key))
		case key == "lang" && strings.TrimSpace(s) == "":
			// The page's <html lang> is the data's own, so that every way of drawing it gives the same page.
			found = append(found, yamlpos.New(d.File, d.Where[key], "doc_no_lang"))
		case ok:
			*to[key] = s
		}
	}
	// A title of spaces names none.
	d.Title = strings.TrimSpace(d.Title)
	return found.Sorted().Err()
}
