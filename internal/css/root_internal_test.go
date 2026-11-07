package css

import (
	"slices"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
)

// rootAtRules are the rules a declaration on a root may stand in and still be on it: they choose when it holds, not
// which element it is on.
var rootAtRules = []string{"@media", "@supports", "@layer", "@container"}

// rootNarrowing are the pseudo-classes that narrow a root on the same element, as floe's CSS writes them; what they
// hold is not read.
var rootNarrowing = []string{"has(", "not(", "is(", "where("}

// Off says why a declaration's rules do not set it on a root, "" when they do.
type Off string

const (
	OnTheRoot Off = ""
	// Nested is a rule within a rule (CSS nesting): it is read as on another element.
	Nested Off = "nested"
	// AtRule is a rule out of rootAtRules around it (@scope makes :scope another element).
	AtRule Off = "at-rule"
	// Inner is a selector that reaches past the root, or one not read as narrowing it.
	Inner Off = "inner"
)

// trimmed returns the tokens of src without the whitespace at its ends.
func trimmed(src string) []token {
	toks := lex([]byte(src))
	for len(toks) > 0 && toks[0].tt == css.WhitespaceToken {
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].tt == css.WhitespaceToken {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// closing returns the place of the token that closes the bracket opened at toks[i] (a function or ( with ), [ with
// ]), or -1 when none does: each bracket closes on its own kind, as CSS Syntax nests them.
func closing(toks []token, i int) int {
	var open []css.TokenType
	for j := i; j < len(toks); j++ {
		switch toks[j].tt {
		case css.FunctionToken, css.LeftParenthesisToken:
			open = append(open, css.RightParenthesisToken)
		case css.LeftBracketToken:
			open = append(open, css.RightBracketToken)
		case css.RightParenthesisToken, css.RightBracketToken:
			if len(open) == 0 || open[len(open)-1] != toks[j].tt {
				return -1
			}
			if open = open[:len(open)-1]; len(open) == 0 {
				return j
			}
		default:
		}
	}
	return -1
}

// RootOf says whether rule, the preludes of a declaration's rules joined by " { ", sets it on root (":scope" or
// ":root"), by a context-free rule on the tokens: within rootAtRules or none, the last prelude is root followed by
// classes, attributes and the pseudo-classes of rootNarrowing alone; anything else is not the root. Letters compare as
// CSS compares them, in either case.
func RootOf(rule, root string) Off {
	preludes := strings.Split(rule, " { ")
	for _, p := range preludes[:len(preludes)-1] {
		toks := trimmed(p)
		switch {
		case len(toks) > 0 && toks[0].tt == css.AtKeywordToken && slices.Contains(rootAtRules, strings.ToLower(string(toks[0].data))):
		case len(toks) > 0 && toks[0].tt == css.AtKeywordToken:
			return AtRule
		default:
			return Nested
		}
	}
	toks := trimmed(preludes[len(preludes)-1])
	name := strings.TrimPrefix(root, ":")
	if len(toks) < 2 || toks[0].tt != css.ColonToken || toks[1].tt != css.IdentToken || !strings.EqualFold(string(toks[1].data), name) {
		return Inner
	}
	for i := 2; i < len(toks); i++ {
		switch {
		case toks[i].tt == css.DelimToken && string(toks[i].data) == "." && i+1 < len(toks) && toks[i+1].tt == css.IdentToken:
			i++
		case toks[i].tt == css.LeftBracketToken:
			if i = closing(toks, i); i < 0 {
				return Inner
			}
		case toks[i].tt == css.ColonToken && i+1 < len(toks) && toks[i+1].tt == css.FunctionToken &&
			slices.Contains(rootNarrowing, strings.ToLower(string(toks[i+1].data))):
			if i = closing(toks, i+1); i < 0 {
				return Inner
			}
		default:
			return Inner
		}
	}
	return OnTheRoot
}

func TestRootOf(t *testing.T) {
	t.Parallel()

	for rule, want := range map[string]Off{
		":scope": OnTheRoot, ":SCOPE": OnTheRoot, ":scope.numbered": OnTheRoot, ":scope:has(> .option > .parts)": OnTheRoot,
		":scope[open]": OnTheRoot, ":scope:has(> a):not(:has(> b))": OnTheRoot, ":scope:IS(.a)": OnTheRoot,
		"@media (max-width: 36rem) { :scope": OnTheRoot, "@supports (x) { :scope:has(> a)": OnTheRoot,
		"@container (width > 1px) { :scope": OnTheRoot, "@Media print { :scope": OnTheRoot, "@layer a { :scope": OnTheRoot,
		":scope > li": Inner, ":scope li": Inner, ":scope\n  li": Inner, ":scope/**/li": Inner, ":scope+li": Inner,
		":scope, .card": Inner, ":scope::before": Inner, ":scope:before": Inner, ":scope:hover": Inner, ":scope.": Inner,
		".card": Inner, ":scopes": Inner, ":root": Inner, ":is(:scope)": Inner, "@media print { :scope > li": Inner,
		":scope { &.numbered": Nested, ":scope { @media print { :scope": Nested, "li { :scope": Nested,
		"@scope (li) { :scope": AtRule, "@starting-style { :scope": AtRule, "@-webkit-x { :scope": AtRule,
		`@m\edia print { :scope`: AtRule, ":scope:has(> li": Inner, ":scope[open": Inner, ":scope:has(> li]": Inner, ":scope[open)": Inner,
	} {
		if got := RootOf(rule, ":scope"); got != want {
			t.Errorf("RootOf(%q) = %q, want %q", rule, got, want)
		}
	}
	if RootOf(":root", ":root") != OnTheRoot || RootOf(":scope", ":root") != Inner {
		t.Error(":root is read as :scope is")
	}
}
