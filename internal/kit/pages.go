package kit

import (
	"html/template"
	"maps"
	"path"

	"github.com/sukekyo26/floe/internal/usererr"
)

// pages are the HTML of a part or a template parsed, a page for each language of its words, and one, "", for
// a page of a language it has no words in.
type pages map[string]*template.Template

// parsePages parses src, the HTML of file (part.html or page.html), with fns, and with word for each language
// of w: html/template takes the functions before it parses. Its mistake is text/template's, in its words.
// LIMIT: text/template writes the line in its words, as it gives no place apart.
func parsePages(file string, src []byte, fns template.FuncMap, w words) (pages, error) {
	base, err := template.New(path.Base(file)).Option("missingkey=error").Funcs(fns).
		Funcs(template.FuncMap{"word": w.word(file, "")}).Parse(string(src))
	if err != nil {
		return nil, usererr.LibraryAt(file, 0, 0, err.Error())
	}
	ps := pages{"": base}
	for lang := range maps.Keys(w) {
		page, err := base.Clone()
		if err != nil {
			return nil, err //nolint:wrapcheck // floe's failure: a template not yet run clones
		}
		ps[lang] = page.Funcs(template.FuncMap{"word": w.word(file, lang)})
	}
	return ps, nil
}

// page returns the page of ps for the language tag, as w picks it.
func (ps pages) page(w words, tag string) *template.Template {
	return ps[w.lang(tag)]
}
