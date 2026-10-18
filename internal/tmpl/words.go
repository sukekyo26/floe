package tmpl

import (
	"errors"
	"slices"
	"strings"
	tparse "text/template/parse"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// words are the words a part writes in the page's language (the headings of risks), by the language tag,
// lowercased, and their names: a part's words.yaml. A part without one has en alone, without words.
type words map[string]map[string]string

// fallbackLang is the language of words a page of a language a part has none of is drawn in.
const fallbackLang = "en"

// readWords reads src, the words.yaml of a part, file in its mistakes: language tags, en among them,
// each the names of the words and their text, every language naming the words en names. One without a
// document has no words.
func readWords(file string, src []byte) (words, error) {
	body, err := yamlpos.Parse(file, src, "tmpl_words_documents")
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
	// Each language as written, and where, by its lowercased tag.
	written := map[string]string{}
	where := map[string]yamlpos.Pos{}
	var problems yamlpos.Problems
	for _, v := range top.Values {
		key := yamlpos.KeyOf(v)
		tag := key.GetToken().Value
		lang := strings.ToLower(tag)
		switch {
		case !i18n.IsTag(tag):
			problems = append(problems, yamlpos.New(file, yamlpos.Of(key), "tmpl_words_lang", tag))
			continue
		case written[lang] != "":
			// A tag's case means nothing: EN after en would take its place unseen.
			problems = append(problems, yamlpos.New(file, yamlpos.Of(key), "tmpl_words_lang_twice", tag, written[lang]))
			continue
		}
		written[lang], where[lang] = tag, yamlpos.Of(key)
		ws, more := readLang(file, tag, v.Value)
		w[lang], problems = ws, append(problems, more...)
	}
	if problems != nil {
		return nil, problems.Sorted()
	}
	return w, w.checkNames(file, written, where, yamlpos.Of(top))
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

// checkNames refuses w without en, at top, and a language of w, written as written and at where, that
// leaves out a word en names or names one en does not.
func (w words) checkNames(file string, written map[string]string, where map[string]yamlpos.Pos, top yamlpos.Pos) error {
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
			problems = append(problems, yamlpos.New(file, where[lang], "tmpl_words_missing", written[lang], andList(missing)))
		}
		if extra != nil {
			slices.Sort(extra)
			problems = append(problems, yamlpos.New(file, where[lang], "tmpl_words_extra", written[lang], andList(extra)))
		}
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// lang picks the language of w a page of the language tag is drawn in, as i18n.Lookup finds it, else en.
func (w words) lang(tag string) string {
	if lang, ok := i18n.Lookup(tag, func(t string) bool { _, ok := w[t]; return ok }); ok {
		return lang
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

// checkWords refuses a word page names as written (word "likely") that w does not have, so that a
// misspelt one stops the part when it is read, not a page that happens to take that branch; html and
// file name the part's HTML and its words in the mistake. A word named from the data (word $level) is
// checked when it is drawn.
func (w words) checkWords(page *tparse.Tree, html, file string) error {
	var names []string
	wordNames(page.Root, &names)
	for _, name := range names {
		if _, ok := w[fallbackLang][name]; !ok {
			return usererr.New("tmpl_words_unnamed", html, name, file)
		}
	}
	return nil
}

// wordNames adds to names the names of the words n and the nodes under it name as written.
func wordNames(n tparse.Node, names *[]string) {
	if c, ok := n.(*tparse.CommandNode); ok && len(c.Args) > 1 {
		id, isIdent := c.Args[0].(*tparse.IdentifierNode)
		if s, isText := c.Args[1].(*tparse.StringNode); isIdent && isText && id.Ident == "word" {
			*names = append(*names, s.Text)
		}
	}
	for _, c := range children(n) {
		wordNames(c, names)
	}
}

// children returns the nodes right under n that may hold a call of word.
func children(n tparse.Node) []tparse.Node {
	switch n := n.(type) {
	case *tparse.ListNode:
		if n != nil {
			return n.Nodes
		}
	case *tparse.PipeNode:
		if n != nil {
			nodes := make([]tparse.Node, 0, len(n.Cmds))
			for _, c := range n.Cmds {
				nodes = append(nodes, c)
			}
			return nodes
		}
	case *tparse.ActionNode:
		return []tparse.Node{n.Pipe}
	case *tparse.TemplateNode:
		return []tparse.Node{n.Pipe}
	case *tparse.CommandNode:
		return n.Args
	case *tparse.IfNode:
		return []tparse.Node{n.Pipe, n.List, n.ElseList}
	case *tparse.RangeNode:
		return []tparse.Node{n.Pipe, n.List, n.ElseList}
	case *tparse.WithNode:
		return []tparse.Node{n.Pipe, n.List, n.ElseList}
	}
	return nil
}
