package tmpl

import (
	"errors"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// words are the words a part writes in the page's language (the headings of risks), by the language tag,
// lowercased, and their names: a part's words.yaml. nil for a part without one.
type words map[string]map[string]string

// fallbackLang is the language of words a page of a language a part has none of is drawn in.
const fallbackLang = "en"

// readWords reads src, the words.yaml of a part, file in its mistakes: language tags, en among them,
// each the names of the words and their text, every language naming the words en names. One without a
// document has no words.
func readWords(file string, src []byte) (words, error) {
	body, err := yamlpos.Parse(file, src, "tmpl_words_yaml")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return nil, nil //nolint:nilnil // no words
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // a mistake at its line and column
	}
	if err = plain(file, body); err != nil {
		return nil, err
	}
	top, ok := body.(*ast.MappingNode)
	if !ok {
		return nil, yamlpos.New(file, yamlpos.Of(body), "tmpl_words_yaml")
	}
	w := words{}
	where := map[string]yamlpos.Pos{}
	var problems yamlpos.Problems
	for _, v := range top.Values {
		key := yamlpos.KeyOf(v)
		tag := key.GetToken().Value
		if !i18n.IsTag(tag) {
			problems = append(problems, yamlpos.New(file, yamlpos.Of(key), "tmpl_words_lang", tag))
			continue
		}
		lang := strings.ToLower(tag)
		where[lang] = yamlpos.Of(key)
		ws, more := readLang(file, tag, v.Value)
		w[lang], problems = ws, append(problems, more...)
	}
	if problems != nil {
		return nil, problems.Sorted()
	}
	return w, w.checkNames(file, where, yamlpos.Of(top))
}

// readLang reads n, the words of the language tag: names and their text.
func readLang(file, tag string, n ast.Node) (map[string]string, yamlpos.Problems) {
	m, ok := n.(*ast.MappingNode)
	if !ok {
		return nil, yamlpos.Problems{yamlpos.New(file, yamlpos.Of(n), "tmpl_words_lang_not_mapping", tag)}
	}
	ws := map[string]string{}
	var problems yamlpos.Problems
	for _, v := range m.Values {
		name := yamlpos.KeyOf(v).GetToken().Value
		text, ok := v.Value.(*ast.StringNode)
		if !ok {
			problems = append(problems, yamlpos.New(file, yamlpos.Of(v.Value), "tmpl_words_not_text", tag, name))
			continue
		}
		ws[name] = text.Value
	}
	return ws, problems
}

// checkNames refuses w without en, at top, and a language of w, written at where, that leaves out a word
// en names or names one en does not.
func (w words) checkNames(file string, where map[string]yamlpos.Pos, top yamlpos.Pos) error {
	en, ok := w[fallbackLang]
	if !ok {
		return yamlpos.New(file, top, "tmpl_words_no_en")
	}
	var problems yamlpos.Problems
	for lang, ws := range w {
		var missing, extra []string
		for name := range en {
			if _, ok := ws[name]; !ok {
				missing = append(missing, name)
			}
		}
		for name := range ws {
			if _, ok := en[name]; !ok {
				extra = append(extra, name)
			}
		}
		if missing != nil {
			slices.Sort(missing)
			problems = append(problems, yamlpos.New(file, where[lang], "tmpl_words_missing", lang, andList(missing)))
		}
		if extra != nil {
			slices.Sort(extra)
			problems = append(problems, yamlpos.New(file, where[lang], "tmpl_words_extra", lang, andList(extra)))
		}
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// lang picks the language of w a page of the language tag is drawn in: the tag itself, else its
// language without the region, else en.
func (w words) lang(tag string) string {
	tag = strings.ToLower(tag)
	if _, ok := w[tag]; ok {
		return tag
	}
	if base, _, found := strings.Cut(tag, "-"); found {
		if _, ok := w[base]; ok {
			return base
		}
	}
	return fallbackLang
}

// word returns the function word of a part's HTML in the language lang of w: the text of a word by its
// name. A name w does not have stops the drawing, file naming where the words are.
func (w words) word(file, lang string) func(string) (string, error) {
	return func(name string) (string, error) {
		if text, ok := w[lang][name]; ok {
			return text, nil
		}
		return "", usererr.New("tmpl_words_unknown", file, name)
	}
}
