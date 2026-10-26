package kit

import (
	"cmp"
	"errors"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// words are the words a part or a template writes in the page's language (the headings of risks), by the
// language tag, lowercased, and their names: its words.yaml; nil without one.
type words map[string]map[string]string

// readWords reads src, the words.yaml of a part or a template, file in its mistakes: language tags, each the
// names of the words and their text. An empty one has none. Whether a tag is one, en is there, and the
// languages name the same words is not checked: a word a page names that its language does not have stops
// the drawing (word).
func readWords(file string, src []byte) (words, error) {
	body, err := yamlpos.Parse(file, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
		return words{}, nil
	case err != nil:
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, or floe's failure
	}
	top, ok := body.(*ast.MappingNode)
	if !ok {
		return nil, yamlpos.New(file, yamlpos.Of(body), "kit_words_yaml")
	}
	w := words{}
	written := map[string]string{} // the tag of each language as written first
	var found usererr.List
	for _, v := range top.Values {
		key := yamlpos.KeyOf(v)
		tag := key.GetToken().Value
		lang := string(i18n.Tag(tag))
		// Its words are read whatever else is wrong, so that their mistakes are told in the same run.
		ws, more, err := readLang(file, tag, key, v.Value)
		if err != nil {
			return nil, err
		}
		found = append(found, more...)
		if first, twice := written[lang]; twice {
			// A tag's case and _ mean nothing to the lookup: EN after en would take its place unseen.
			found = append(found, yamlpos.New(file, yamlpos.Of(key), "kit_words_lang_twice", tag, first))
			continue
		}
		written[lang] = tag
		w[lang] = ws
	}
	if found != nil {
		return nil, found.Sorted().Err()
	}
	return w, nil
}

// readLang reads n, the words of the language tag written as key: names and their text, or floe's failure.
func readLang(file, tag string, key, n ast.Node) (map[string]string, usererr.List, error) {
	m, ok := n.(*ast.MappingNode)
	if !ok {
		return nil, usererr.List{yamlpos.New(file, yamlpos.OfValue(key, n), "kit_words_lang_not_mapping", tag)}, nil
	}
	ws := map[string]string{}
	var found usererr.List
	for _, v := range m.Values {
		name := yamlpos.KeyOf(v).GetToken().Value
		text, ok, err := yamlpos.Text(v.Value)
		switch {
		case err != nil:
			return nil, nil, err //nolint:wrapcheck // floe's failure, which says what failed
		case ok:
			ws[name] = text
		default:
			at := yamlpos.OfValue(yamlpos.KeyOf(v), v.Value)
			found = append(found, yamlpos.New(file, at, "kit_words_not_text", tag, name))
		}
	}
	return ws, found, nil
}

// lang returns the language of w a page of the language tag is drawn in: the first of i18n.Fallbacks of the
// tag that w has; "" when it has none of them.
func (w words) lang(tag string) string {
	for _, l := range i18n.Fallbacks(i18n.Lang(tag)) {
		if _, ok := w[string(l)]; ok {
			return string(l)
		}
	}
	return ""
}

// word returns the function word of a part's HTML or a template's page.html in the language lang of w ("" for
// a page of a language w has none of, nor en): the text of a word by its name. A name the language does not
// have stops the drawing, file naming the HTML that names it, and the language: en for "", the last of the
// languages a page falls back to.
func (w words) word(file, lang string) func(string) (string, error) {
	return func(name string) (string, error) {
		if text, ok := w[lang][name]; ok {
			return text, nil
		}
		return "", usererr.NewAt(file, 0, 0, "kit_words_unknown", name, cmp.Or(lang, string(i18n.English)))
	}
}
