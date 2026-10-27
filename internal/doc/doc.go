// Package doc reads a data file: its values and where each is written, and the keys every template
// shares (lang, template, theme, title), which floe reads itself.
package doc

import (
	"errors"
	"math"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Data is a data file read: its values as written, and the keys every template shares read from them.
type Data struct {
	File                         string
	Lang, Template, Theme, Title string           // "" for one left out or refused; each trimmed of spaces
	Values                       map[string]any   // as written, the shared keys too but those that name none
	Root                         *ast.MappingNode // nil for an empty file
	// where each shared key is written: its value, its key when the value is left out, the top of the file
	// for one left out; none (the zero Pos) for a theme given
	Where map[string]yamlpos.Pos
}

// PosOf returns where the value at the path at (keys and indexes) is written in d: the value, or its key when
// it is left out; the top of the file without a node.
func (d *Data) PosOf(at []string) yamlpos.Pos {
	if d.Root == nil {
		return yamlpos.Pos{Line: 1, Column: 1}
	}
	return yamlpos.OfValue(yamlpos.Find(d.Root, at))
}

// Shared are the keys of a data file floe reads itself, in every template, which the template's schema neither
// declares nor checks.
var Shared = []string{"lang", "template", "theme", "title"}

// Read reads the data file src, file in its mistakes: yamlpos.Parse, and only when it refused nothing,
// goccy/go-yaml's reading of the values (its mistake for a top that is not keys and values), then the
// shared keys, each text, and lang: written and not empty or spaces. An empty file, or one of comments
// alone, reads as {}, its mistakes at 1:1, as it has no node to place them at. Its whole numbers are made
// int (Ints). A theme given ("" or spaces for none) is drawn with instead of the
// data's: theme: is then neither read nor refused, and Values holds the theme given, read as one written, so
// that every layer sees the theme the page is drawn with.
func Read(file string, src []byte, theme string) (*Data, error) {
	body, err := yamlpos.Parse(file, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
		body = nil
	case err != nil:
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, or floe's failure
	}
	values := map[string]any{}
	if body != nil {
		if err = yamlpos.Decode(file, body, &values); err != nil {
			return nil, err //nolint:wrapcheck // goccy's mistake at its place
		}
		Ints(values)
	}
	// goccy/go-yaml read the values into a map: the top is keys and values, or there is none.
	root, _ := body.(*ast.MappingNode) //nolint:errcheck // see above
	d := &Data{
		File: file, Lang: "", Template: "", Theme: "", Title: "", Values: values, Root: root,
		Where: map[string]yamlpos.Pos{},
	}
	// A theme given is read as one written, spaces naming none.
	given := strings.TrimSpace(theme) != ""
	if given {
		values["theme"] = theme
	}
	return d, d.readShared(given)
}

// Ints turns the whole numbers in v that fit into int, in place, and returns v, so that every layer sees one type of
// them: goccy/go-yaml reads one above 0 as uint64, which expr negates by wrapping around (-3 / 2 is 9.2e18), and
// one below 0 as int64, as expr gives some of its values (date(…).Unix()). A type of its own (time.Duration)
// stays as it is.
// LIMIT: a whole number above 9.2e18 stays uint64, which expr's - still wraps around (-a < 0 does not hold).
func Ints(v any) any {
	switch v := v.(type) {
	case int64:
		return int(v)
	case uint64:
		if v <= math.MaxInt64 {
			return int(v)
		}
	case map[string]any:
		for k, item := range v {
			v[k] = Ints(item)
		}
	case []any:
		for i, item := range v {
			v[i] = Ints(item)
		}
	}
	return v
}

// sharedMistake returns the message key of what is wrong with v, the value of the shared key, written or not
// (ok), "" for none: one written that is not text, and a lang that is none.
func sharedMistake(key string, v any, ok bool) string {
	_, text := v.(string)
	switch {
	case ok && !text:
		return "doc_not_text"
	case key == "lang" && !ok:
		// The page's <html lang> is the data's own, so that every way of drawing it gives the same page.
		return "doc_no_lang"
	}
	return ""
}

// readShared reads the keys every template shares from d.Values into d, leaving them in d.Values as written but
// those null, empty or of spaces, which are none written, and notes where each is written; theme: is not read
// with a theme given, which d takes.
func (d *Data) readShared(themeGiven bool) error {
	top := yamlpos.Pos{Line: 1, Column: 1}
	pairs := map[string]*ast.MappingValueNode{}
	if d.Root != nil {
		top = yamlpos.Of(d.Root)
		for _, v := range d.Root.Values {
			pairs[yamlpos.KeyName(v)] = v
		}
	}
	to := map[string]*string{"lang": &d.Lang, "template": &d.Template, "theme": &d.Theme, "title": &d.Title}
	var found usererr.List
	for _, key := range Shared {
		d.Where[key] = top
		if p := pairs[key]; p != nil {
			d.Where[key] = yamlpos.OfValue(yamlpos.KeyOf(p), p.Value)
		}
		v, ok := d.Values[key]
		if s, text := v.(string); ok && (v == nil || text && strings.TrimSpace(s) == "") {
			// Null, empty or spaces (lang:, lang: ~, lang: "") is none written, for every reader of d.Values:
			// these are keys floe reads itself, while the other values are the schema's, which tells a null as it is.
			delete(d.Values, key)
			v, ok = nil, false
		}
		mistake := sharedMistake(key, v, ok)
		switch {
		case mistake == "doc_no_lang":
			found = append(found, yamlpos.New(d.File, d.Where[key], mistake))
		case mistake != "":
			found = append(found, yamlpos.New(d.File, d.Where[key], mistake, key))
		case ok:
			s, _ := v.(string) //nolint:errcheck // sharedMistake refuses one that is not text
			*to[key] = strings.TrimSpace(s)
			d.Values[key] = *to[key]
		}
	}
	if themeGiven {
		d.Where["theme"] = yamlpos.Pos{} // no line of d names it
	}
	// Read once as every lookup of it and the page's <html lang> take it: BCP 47 in lower case, JA and ja_JP ja's.
	d.Lang = string(i18n.Tag(d.Lang))
	return found.Sorted().Err()
}
