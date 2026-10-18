package tmpl

import (
	"errors"
	"html/template"
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
	seen := map[string]writtenTag{}
	var problems yamlpos.Problems
	for _, v := range top.Values {
		key := yamlpos.KeyOf(v)
		tag := key.GetToken().Value
		lang := strings.ToLower(tag)
		switch {
		case !i18n.IsTag(tag):
			problems = append(problems, yamlpos.New(file, yamlpos.Of(key), "tmpl_words_lang", tag))
			continue
		case seen[lang].tag != "":
			// A tag's case means nothing: EN after en would take its place unseen.
			problems = append(problems, yamlpos.New(file, yamlpos.Of(key), "tmpl_words_lang_twice", tag, seen[lang].tag))
			continue
		}
		seen[lang] = writtenTag{tag: tag, at: yamlpos.Of(key)}
		ws, more := readLang(file, tag, v.Value)
		w[lang], problems = ws, append(problems, more...)
	}
	// The names are checked beside the other mistakes, so that one run tells them all.
	if problems = append(problems, w.checkNames(file, seen, yamlpos.Of(top))...); problems != nil {
		return nil, problems.Sorted()
	}
	return w, nil
}

// writtenTag is a language tag of words.yaml as written, and where.
type writtenTag struct {
	tag string
	at  yamlpos.Pos
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
		switch text := v.Value.(type) {
		case *ast.StringNode:
			ws[name] = text.Value
		case *ast.LiteralNode: // | or >, its line break at the end dropped: a word is drawn inline
			ws[name] = strings.TrimRight(text.Value.Value, "\n")
		default:
			// Named all the same, so that it is not told left out too.
			ws[name] = ""
			problems = append(problems, yamlpos.New(file, yamlpos.Of(v.Value), "tmpl_words_not_text", tag, name))
		}
	}
	return ws, problems
}

// checkNames refuses w without en, at top, and a language of w, written as seen says, that leaves out a
// word en names or names one en does not.
func (w words) checkNames(file string, seen map[string]writtenTag, top yamlpos.Pos) yamlpos.Problems {
	en, ok := w[fallbackLang]
	if !ok {
		return yamlpos.Problems{yamlpos.New(file, top, "tmpl_words_no_en")}
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
			problems = append(problems, yamlpos.New(file, seen[lang].at, "tmpl_words_missing", seen[lang].tag, andList(missing)))
		}
		if extra != nil {
			slices.Sort(extra)
			problems = append(problems, yamlpos.New(file, seen[lang].at, "tmpl_words_extra", seen[lang].tag, andList(extra)))
		}
	}
	return problems
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

// checkWords refuses a word page, and the templates it defines, name as written (word "likely") that w
// does not have, so that a misspelt one stops the part when it is read, not a page that happens to take
// that branch; html and file name the part's HTML and its words in the mistake. A word named from the
// data (word $level) is checked when it is drawn.
func (w words) checkWords(page *template.Template, html, file string) error {
	var missing []string
	for _, name := range namedWords(page) {
		if _, ok := w[fallbackLang][name]; !ok {
			missing = append(missing, name)
		}
	}
	if missing != nil {
		return usererr.New("tmpl_words_unnamed", html, andList(missing), file)
	}
	return nil
}

// namedWords returns the names of the words page, and the templates it defines, name as written, once
// each, in order.
func namedWords(page *template.Template) []string {
	var names []string
	for _, t := range page.Templates() {
		if t.Tree != nil {
			wordNames(t.Tree.Root, &names)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// wordNames adds to names the names of the words n and the nodes under it name as written: word "x",
// or "x" | word.
// LIMIT: a name in parentheses (word ("x")) or a chain ((word "x").y) is left to the drawing; follow
// those nodes too if a part comes to write one so.
func wordNames(n tparse.Node, names *[]string) {
	switch n := n.(type) {
	case *tparse.CommandNode:
		if isWord(n) && len(n.Args) > 1 {
			*names = append(*names, literal(n.Args[1])...)
		}
	case *tparse.PipeNode:
		for i := 1; n != nil && i < len(n.Cmds); i++ {
			if c := n.Cmds[i]; isWord(c) && len(c.Args) == 1 && len(n.Cmds[i-1].Args) == 1 {
				*names = append(*names, literal(n.Cmds[i-1].Args[0])...)
			}
		}
	}
	for _, c := range children(n) {
		wordNames(c, names)
	}
}

// literal returns the text of n, a string as written, or none.
func literal(n tparse.Node) []string {
	if s, ok := n.(*tparse.StringNode); ok {
		return []string{s.Text}
	}
	return nil
}

// isWord reports whether c calls word.
func isWord(c *tparse.CommandNode) bool {
	id, ok := c.Args[0].(*tparse.IdentifierNode)
	return ok && id.Ident == "word"
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
