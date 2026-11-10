package lint

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"

	floecss "floe/internal/css"
)

// token is a token of floecss.Tokens, which every rule reads by: it reads no CSS of its own.
type token = floecss.Token

// decl is a declaration: its value by its tokens, whitespace and comments left out and !important taken off its end, the
// preludes of the rules it is in, outermost first, joined by " { " ("" outside every rule), the place of the { that holds
// it right (-1 outside every block: two rules of one prelude are two blocks), and the byte its name starts at.
type decl struct {
	value     []token
	important bool
	rule      string
	block     int
	at        int
}

// rules returns, for each token, the preludes of the rules it is in, outermost first and joined by " { ", comments left
// out, the place of the innermost { of a rule it is in (-1 outside every rule), and whether it is a rule's {, ;, or the }
// that closes a rule. A rule's { is one with no ( nor [ around it in its rule (its Depth is the rules it is in); a {, ;
// or } within a function or a bracket is a token of its value.
func rules(toks []token) ([]string, []int, []bool) {
	in, blocks, ruling := make([]string, len(toks)), make([]int, len(toks)), make([]bool, len(toks))
	var open []string
	var at []int
	var prelude strings.Builder
	for k, t := range toks {
		switch {
		case t.Type == css.LeftBraceToken && t.Depth == len(open):
			open = append(open, strings.TrimSpace(prelude.String()))
			at = append(at, k)
			prelude.Reset()
			ruling[k] = true
		case t.Type == css.RightBraceToken && len(open) > 0 && t.Depth == len(open)-1:
			open, at = open[:len(open)-1], at[:len(at)-1]
			prelude.Reset()
			ruling[k] = true
		case t.Type == css.SemicolonToken && t.Depth == len(open):
			prelude.Reset()
			ruling[k] = true
		case t.Type == css.CommentToken:
		default:
			prelude.Write(t.Data)
		}
		blocks[k] = -1
		if len(open) > 0 {
			in[k], blocks[k] = strings.Join(open, " { "), at[len(at)-1]
		}
	}
	return in, blocks, ruling
}

// skipped is whether a token is one CSS Syntax drops between a declaration's parts: whitespace or a comment.
func skipped(t token) bool { return t.Type == css.WhitespaceToken || t.Type == css.CommentToken }

// declarations returns each declaration in src of a property that want takes, by name: a name at the start of a rule's
// block or after its ; (rules), a colon (whitespace and comments before it, as CSS Syntax drops them), and the value up to the ; or } of the
// name's depth. What reaches a { of that depth is a rule's prelude (li:has(…) {), not a declaration. A property's name is
// lowered, a variable's kept as written: CSS tells --Radius from --radius.
func declarations(src []byte, want func(name string) bool) map[string][]decl {
	found := map[string][]decl{}
	toks := tokensOf(src)
	in, blocks, ruling := rules(toks)
	start := true
	for i := 0; i < len(toks); i++ {
		switch {
		case skipped(toks[i]):
			continue
		case ruling[i]:
			start = true
			continue
		}
		begins := start
		start = false
		name := string(toks[i].Data)
		if !strings.HasPrefix(name, "--") {
			name = strings.ToLower(name)
		}
		if !begins || (toks[i].Type != css.IdentToken && toks[i].Type != css.CustomPropertyNameToken) || !want(name) {
			continue
		}
		depth := toks[i].Depth
		j := i + 1
		for j < len(toks) && skipped(toks[j]) {
			j++
		}
		if j == len(toks) || toks[j].Type != css.ColonToken {
			continue
		}
		var value []token
		prelude := false
		for j++; j < len(toks); j++ {
			t := toks[j]
			if t.Depth == depth && t.Type == css.SemicolonToken || t.Depth < depth {
				break // a ; of the name's depth, or the } that closes its block
			}
			if t.Depth == depth && t.Type == css.LeftBraceToken {
				prelude = true
				break
			}
			if !skipped(t) {
				value = append(value, t)
			}
		}
		if prelude {
			continue
		}
		important := false
		if n := len(value); n >= 2 && value[n-2].Type == css.DelimToken && string(value[n-2].Data) == "!" &&
			value[n-1].Type == css.IdentToken && strings.EqualFold(string(value[n-1].Data), "important") {
			value, important = value[:n-2], true
		}
		found[name] = append(found[name], decl{value: value, important: important, rule: in[i], block: blocks[i], at: toks[i].At})
		i = j - 1
	}
	return found
}

// tokensOf returns the tokens of src, by floecss.Tokens.
func tokensOf(src []byte) []token { return floecss.Tokens(src) }

// text returns the value as src writes it, from its first token to its last.
func text(src []byte, value []token) string {
	if len(value) == 0 {
		return ""
	}
	last := value[len(value)-1]
	return string(src[value[0].At : last.At+len(last.Data)])
}

// lineOf returns the line of src the byte at is on, from 1.
func lineOf(src []byte, at int) int {
	return bytes.Count(src[:at], []byte("\n")) + 1
}

// opener is whether t opens a block or a function.
func opener(t token) bool {
	return t.Type == css.FunctionToken || t.Type == css.LeftParenthesisToken || t.Type == css.LeftBracketToken ||
		t.Type == css.LeftBraceToken
}

// items splits a value into its items at its own depth: a function or a bracket with what it holds, or a token.
func items(value []token) [][]token {
	if len(value) == 0 {
		return nil
	}
	base := value[0].Depth
	var out [][]token
	start := -1
	for i, t := range value {
		switch {
		case t.Depth > base:
		case start >= 0:
			out = append(out, value[start:i+1])
			start = -1
		case opener(t):
			start = i
		default:
			out = append(out, value[i:i+1])
		}
	}
	if start >= 0 {
		out = append(out, value[start:])
	}
	return out
}

// joined returns each item of value as its tokens written together, without what was between them.
func joined(value []token) []string {
	all := items(value)
	out := make([]string, 0, len(all))
	for _, item := range all {
		var b strings.Builder
		for _, t := range item {
			b.Write(t.Data)
		}
		out = append(out, b.String())
	}
	return out
}

// cutSlash splits a value at its / of the value's own depth.
func cutSlash(value []token) (before, after []token, ok bool) {
	for i, t := range value {
		if t.Depth == value[0].Depth && t.Type == css.DelimToken && string(t.Data) == "/" {
			return value[:i], value[i+1:], true
		}
	}
	return value, nil, false
}

// reads returns the variables src reads by var(), its first argument.
func reads(src []byte) map[string]bool {
	found := map[string]bool{}
	toks := tokensOf(src)
	for i, t := range toks {
		if t.Type != css.FunctionToken || !strings.EqualFold(string(t.Data), "var(") {
			continue
		}
		for j := i + 1; j < len(toks); j++ {
			if skipped(toks[j]) {
				continue
			}
			if toks[j].Type == css.CustomPropertyNameToken || toks[j].Type == css.IdentToken && strings.HasPrefix(string(toks[j].Data), "--") {
				found[string(toks[j].Data)] = true
			}
			break
		}
	}
	return found
}

// rootAtRules are the rules a declaration on a root may stand in and still be on it: they choose when it holds, not
// which element it is on.
var rootAtRules = []string{"@media", "@supports", "@layer", "@container"}

// rootNarrowing are the pseudo-classes that narrow a root on the same element; what they hold is not read.
var rootNarrowing = []string{"has(", "not(", "is(", "where("}

// off says why a declaration's rules do not set it on a root, "" when they do.
type off string

const (
	onTheRoot off = ""
	// nested is a rule within a rule (CSS nesting): it is read as on another element.
	nested off = "nested"
	// atRule is a rule out of rootAtRules around it (@scope makes :scope another element).
	atRule off = "at-rule"
	// inner is a selector that reaches past the root, or one not read as narrowing it.
	inner off = "inner"
)

// trimmed returns the tokens of src without the whitespace and comments at its ends.
func trimmed(src string) []token {
	toks := tokensOf([]byte(src))
	for len(toks) > 0 && skipped(toks[0]) {
		toks = toks[1:]
	}
	for len(toks) > 0 && skipped(toks[len(toks)-1]) {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// closing returns the place of the token that closes the opener at toks[i], or -1 when none does.
func closing(toks []token, i int) int {
	for j := i + 1; j < len(toks); j++ {
		if toks[j].Depth == toks[i].Depth && !opener(toks[j]) {
			if toks[j].Type == css.RightParenthesisToken || toks[j].Type == css.RightBracketToken {
				return j
			}
			return -1
		}
	}
	return -1
}

// rootOf says whether rule, the preludes of a declaration's rules joined by " { ", sets it on root (":scope" or ":root"),
// by a context-free rule on the tokens: within rootAtRules or none, the last prelude is root followed by classes,
// attributes and the pseudo-classes of rootNarrowing alone; anything else is not the root. Each form narrows the same
// element, so what is not the root is never taken for it; a root written otherwise is refused, and is written again so.
// Letters compare as CSS compares them, in either case.
func rootOf(rule, root string) off {
	preludes := strings.Split(rule, " { ")
	for _, p := range preludes[:len(preludes)-1] {
		toks := trimmed(p)
		switch {
		case len(toks) > 0 && toks[0].Type == css.AtKeywordToken && slices.Contains(rootAtRules, strings.ToLower(string(toks[0].Data))):
		case len(toks) > 0 && toks[0].Type == css.AtKeywordToken:
			return atRule
		default:
			return nested
		}
	}
	toks := trimmed(preludes[len(preludes)-1])
	name := strings.TrimPrefix(root, ":")
	if len(toks) < 2 || toks[0].Type != css.ColonToken || toks[1].Type != css.IdentToken || !strings.EqualFold(string(toks[1].Data), name) {
		return inner
	}
	for i := 2; i < len(toks); i++ {
		switch {
		case toks[i].Type == css.DelimToken && string(toks[i].Data) == "." && i+1 < len(toks) && toks[i+1].Type == css.IdentToken:
			i++
		case toks[i].Type == css.LeftBracketToken:
			if i = closing(toks, i); i < 0 {
				return inner
			}
		case toks[i].Type == css.ColonToken && i+1 < len(toks) && toks[i+1].Type == css.FunctionToken &&
			slices.Contains(rootNarrowing, strings.ToLower(string(toks[i+1].Data))):
			if i = closing(toks, i+1); i < 0 {
				return inner
			}
		default:
			return inner
		}
	}
	return onTheRoot
}

// rootFix says how to bring a declaration in rule onto root, where the rule's file is for the part or look name ("" for
// none), "" when it is there; inner says it for a selector that reaches past the root.
func rootFix(rule, root, name, inner string) string {
	where := root
	if name != "" {
		where = name + "'s " + root
	}
	switch rootOf(rule, root) {
	case onTheRoot:
		return ""
	case atRule:
		return "move it out of that rule: only @media, @supports, @layer and @container keep it on the root"
	case nested:
		preludes := strings.Split(rule, " { ")
		last := strings.TrimSpace(preludes[len(preludes)-1])
		rest, amp := strings.CutPrefix(last, "&")
		switch {
		case strings.HasPrefix(last, "@"):
			return "write it as " + last + " { " + root + " { … } }"
		case amp && rootOf(root+rest, root) == onTheRoot:
			return "write it unnested, as " + root + rest + " { … }"
		default:
			return "write it unnested, on " + where
		}
	default:
		return inner
	}
}

// sizeOf returns the size a declaration of prop (font-size or font) writes, and whether it writes one, read as its items:
// inherit, in any case, as inherit; font-size's value, none when empty; a font shorthand's item before its / of the
// value's depth, or else its first length, percentage or function, past an angle (oblique's), none when there is none (a
// system font, a keyword's size, CSS's own keywords but inherit, a number alone).
// LIMIT: it reads the font shorthand by hand, not by a parser of its grammar: an angle in a function (oblique calc(10deg),
// var(--slant)) is read as the size, which refuses what is right; floe's CSS writes none. Pass over it if one comes.
func sizeOf(prop string, value []token) (string, bool) {
	all := joined(value)
	if len(all) == 1 && strings.EqualFold(all[0], "inherit") {
		return "inherit", true
	}
	if prop == "font-size" {
		return strings.Join(all, ""), len(all) > 0
	}
	if before, _, ok := cutSlash(value); ok {
		b := joined(before)
		if len(b) == 0 {
			return "", false
		}
		return b[len(b)-1], true
	}
	for _, item := range items(value) {
		t := item[0]
		switch {
		case opener(t):
			return strings.Join(joined(item), ""), true
		case (t.Type == css.DimensionToken || t.Type == css.PercentageToken) && !angle(string(t.Data)):
			return string(t.Data), true
		}
	}
	return "", false
}

// angleUnits are CSS Values' units of an angle, which a font shorthand writes after oblique.
var angleUnits = []string{"deg", "grad", "rad", "turn"}

// angle reports whether the dimension v is in a unit of an angle.
func angle(v string) bool {
	return slices.ContainsFunc(angleUnits, func(u string) bool {
		return len(v) > len(u) && strings.EqualFold(v[len(v)-len(u):], u)
	})
}

// large reports whether v is a rem or an em of 1 or more, written as a number alone.
func large(v string) bool {
	for _, unit := range []string{"rem", "em"} {
		if n, ok := strings.CutSuffix(v, unit); ok {
			f, err := strconv.ParseFloat(n, 64)
			return err == nil && f >= 1
		}
	}
	return false
}

// cornerShapes are the corners floe's CSS may write as they are: shapes, not sizes (a square, a circle, a pill's ends).
var cornerShapes = []string{"0", "50%", "999px"}

// TestDeclarations reads declarations as CSS Syntax does: a comment before the colon is dropped, a value ends at a ; of
// its depth or at the } that closes its block, !important is taken off its end, a prelude's colon is no declaration,
// and a ;, { or } within a function or a bracket is the value's, starting no declaration and closing no rule.
func TestDeclarations(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		".a { opacity: .5; }":                           ".5 false .a",
		".a { opacity/**/: .5 }":                        ".5 false .a",
		".a { opacity /* x */ : .5 } .b { x: 1 }":       ".5 false .a",
		".a { opacity: 0 ! IMPORTANT; }":                "0 true .a",
		".a { opacity: 0!important }":                   "0 true .a",
		".a { b { c: 1 } opacity: .5 }":                 ".5 false .a",
		"@media print { .a { opacity: 1 } }":            "1 false @media print { .a",
		".a { &:hover { opacity: 1; } }":                "1 false .a { &:hover",
		"li:has(a[x=';']) { opacity: 1 }":               "1 false li:has(a[x=';'])",
		":scope { --x: f(}); opacity: .5 }":             ".5 false :scope",
		":scope { --x: f(a; opacity: 1); opacity: .5 }": ".5 false :scope",
		":scope { --x: [a; opacity: 1]; opacity: .5 }":  ".5 false :scope",
	} {
		var got []string
		for _, d := range declarations([]byte(src), func(p string) bool { return p == "opacity" })["opacity"] {
			got = append(got, text([]byte(src), d.value)+" "+map[bool]string{true: "true", false: "false"}[d.important]+" "+d.rule)
		}
		if strings.Join(got, "|") != want {
			t.Errorf("%s: %q, want %q", src, got, want)
		}
	}
	if d := declarations([]byte("a:hover { color: red }"), func(p string) bool { return p == "a" }); len(d) != 0 {
		t.Errorf("a prelude's a:hover is read as a declaration: %v", d)
	}
}

func TestRootOf(t *testing.T) {
	t.Parallel()

	for rule, want := range map[string]off{
		":scope": onTheRoot, ":SCOPE": onTheRoot, ":scope.numbered": onTheRoot, ":scope:has(> .option > .parts)": onTheRoot,
		":scope[open]": onTheRoot, ":scope:has(> a):not(:has(> b))": onTheRoot, ":scope:IS(.a)": onTheRoot,
		"@media (max-width: 36rem) { :scope": onTheRoot, "@supports (x) { :scope:has(> a)": onTheRoot,
		"@container (width > 1px) { :scope": onTheRoot, "@Media print { :scope": onTheRoot, "@layer a { :scope": onTheRoot,
		"/* the root */ :scope /* here */": onTheRoot,
		":scope > li":                      inner, ":scope li": inner, ":scope\n  li": inner, ":scope/**/li": inner, ":scope+li": inner,
		":scope, .card": inner, ":scope::before": inner, ":scope:before": inner, ":scope:hover": inner, ":scope.": inner,
		".card": inner, ":scopes": inner, ":root": inner, ":is(:scope)": inner, "@media print { :scope > li": inner,
		":scope { &.numbered": nested, ":scope { @media print { :scope": nested, "li { :scope": nested,
		"@scope (li) { :scope": atRule, "@starting-style { :scope": atRule, "@-webkit-x { :scope": atRule, "@mediafoo { :scope": atRule,
		`@m\edia print { :scope`: atRule, ":scope:has(> li": inner, ":scope[open": inner, ":scope:has(> li]": inner, ":scope[open)": inner,
	} {
		if got := rootOf(rule, ":scope"); got != want {
			t.Errorf("rootOf(%q) = %q, want %q", rule, got, want)
		}
	}
	if rootOf(":root", ":root") != onTheRoot || rootOf(":scope", ":root") != inner {
		t.Error(":root is read as :scope is")
	}
}

// TestSizeOf reads the size of forms floe's CSS does not write today: none from a value that writes no size, an angle
// passed over, inherit in any case.
func TestSizeOf(t *testing.T) {
	t.Parallel()

	for decl, want := range map[string]string{
		"font-size: ": "", "font-size: /**/": "", "font: caption": "", "font: bold small sans-serif": "",
		"font: initial": "", "font: unset": "", "font: revert": "", "font: revert-layer": "", "font: 0 serif": "",
		"font: 700 var(--floe-text-small)/1.4 sans-serif": "var(--floe-text-small)", "font: oblique 10deg 0.8rem serif": "0.8rem",
		"font: oblique 1.5rad var(--floe-text-aside) serif": "var(--floe-text-aside)", "font: INHERIT": "inherit",
		"font-size: INHERIT": "inherit", "font: 1.1rem serif": "1.1rem", "font-size: small": "small",
		"font-size: initial": "initial", "font: bold small/1.4 serif": "small",
		"font-size: var( --floe-text-small )": "var(--floe-text-small)", "font: var( --floe-text-small ) serif": "var(--floe-text-small)",
		"font-size: 1rem !important": "1rem",
	} {
		src := []byte(":scope { " + decl + "; }")
		prop, _, _ := strings.Cut(decl, ":")
		ds := declarations(src, func(p string) bool { return p == prop })[prop]
		if len(ds) != 1 {
			t.Fatalf("%s: %d declarations", decl, len(ds))
		}
		got, ok := sizeOf(prop, ds[0].value)
		if got != want || ok != (want != "") {
			t.Errorf("sizeOf(%s) = %q, %v; want %q, %v", decl, got, ok, want, want != "")
		}
	}
}

func TestLengthless(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]bool{
		"var(--radius)":                                 true,
		"calc(var(--radius) - var(--floe-room))":        true,
		"min(var(--radius-pill), var(--floe-pill-end))": true,
		"0":                                      true,
		"var(--a, 1px)":                          false,
		"var(--a, var(--b))":                     false,
		"calc(var(--a), var(--b))":               false,
		"min(var(--radius-pill, 4px), var(--b))": false,
		"min(var(--radius-pill), 1rem)":          false,
		"max(var(--a), var(--b))":                false,
	} {
		d := declarations([]byte(".a { border-radius: "+value+"; }"), func(p string) bool { return p == "border-radius" })["border-radius"]
		if got := lengthless(d[0].value); got != want {
			t.Errorf("%s: lengthless %v, want %v", value, got, want)
		}
	}
}

// TestUIParts reads each <style> of the management page from the line it starts on, two of one text too.
func TestUIParts(t *testing.T) {
	t.Parallel()

	ui := floecss.Preprocess([]byte("<style>\r\n.a { color: red }\r\n</style>\r\n<style>\r\n.a { color: red }\r\n</style><p style=\"x: 1\">"))
	sheets, _, err := uiParts(ui)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int, 0, len(sheets))
	for _, s := range sheets {
		got = append(got, s.line)
	}
	if want := []int{0, 3, 5}; !slices.Equal(got, want) {
		t.Errorf("lines before the sheets %v, want %v", got, want)
	}
}
