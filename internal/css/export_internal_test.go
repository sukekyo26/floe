package css

import (
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// Variables returns the names of the variables src sets, in order, for the tests of package css_test.
func Variables(src []byte) []string {
	found := declarations(src, func(p string) bool { return strings.HasPrefix(p, "--") })
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Setting is where a variable is set: the preludes of its rules, joined by " { ", and whether it is !important.
type Setting struct {
	Rule      string
	Important bool
}

// Settings returns, by name, where src sets each variable: a sheet, or the declarations of a style attribute.
func Settings(src []byte) map[string][]Setting {
	found := map[string][]Setting{}
	for name, ds := range declarations(src, func(name string) bool { return strings.HasPrefix(name, "--") }) {
		for _, d := range ds {
			important := false
			for i := 0; i+1 < len(d.value); i++ {
				if d.value[i].tt == css.DelimToken && string(d.value[i].data) == "!" && strings.EqualFold(string(d.value[i+1].data), "important") {
					important = true
				}
			}
			found[name] = append(found[name], Setting{Rule: d.rule, Important: important})
		}
	}
	return found
}

// Read returns the variables src reads by var(), its first argument, comments and whitespace before it left out.
func Read(src []byte) map[string]bool {
	found := map[string]bool{}
	toks := lex(src)
	for i, t := range toks {
		if t.tt != css.FunctionToken || !strings.EqualFold(string(t.data), "var(") {
			continue
		}
		for j := i + 1; j < len(toks); j++ {
			if toks[j].tt == css.WhitespaceToken || toks[j].tt == css.CommentToken {
				continue
			}
			if toks[j].tt == css.CustomPropertyNameToken || toks[j].tt == css.IdentToken && strings.HasPrefix(string(toks[j].data), "--") {
				found[string(toks[j].data)] = true
			}
			break
		}
	}
	return found
}
