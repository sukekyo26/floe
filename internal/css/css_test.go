package css_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/css"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

func told(err error) string {
	if err == nil {
		return ""
	}
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

const (
	brace     = "this } closes no block: floe puts this CSS in a layer, which it would close, leaving the rules after it out of the layer and over the parts' styles"
	imports   = "floe puts this CSS in a layer, where a browser reads no @import: load the stylesheet with a <link> in page.html"
	escape    = `an escape (\) outside a string, a url() or a comment, which floe does not read: write the characters themselves`
	tokenTold = "a browser reads this otherwise than floe: end each string on its own line, and in url() write the address alone or one quoted string"
	unclosed  = "this is still open at the end of the file: floe puts this CSS in a layer, whose } it would take with the rules after it; close it"
	rule      = "palette.css holds only the palette's variables, :root { --name: value; }, and its color-scheme, and no other rule, @media neither: write other rules in base.css or a part's file; for a dark page, make a theme whose :root sets dark colours and color-scheme: dark"
	important = "palette.css takes no !important, which would win over every layer, the colours a template sets on :scope too"
)

func at(file, place, text string) string { return file + ":" + place + ": " + text }

// Check refuses every } that closes no block, @import, escape outside a string and token a browser reads
// otherwise, each where it starts, and passes what a browser reads as floe does.
func TestCheck(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"main { color: red; }\n}\n.a { x: 1 }\n", at("c.css", "2:1", brace)},
		{"@import url(x.css);\nmain{}\n@media print { @import \"y.css\"; }\n", at("c.css", "1:1", imports) + "\n" + at("c.css", "3:16", imports)},
		{"@IMPORT \"x.css\";", at("c.css", "1:1", imports)},
		{"@media print { main { } } }", at("c.css", "1:27", brace)},
		// A } in a function, ( ) or [ ] is the value's, as CSS Syntax reads it.
		{"a { b: f(}) }", ""},
		{"@media (}) { a{} }", ""},
		{"a { b: [}] }", ""},
		{"main::after { content: \"}\" }", ""},
		{"main { color: red } /* } */", ""},
		{"/* @import \"x\"; */ main {}", ""},
		{"main { --x: { a: b }; }", ""},
		{"main { background: url(a}b.png) }", ""},
		{"main { & .a { color: red } }", ""},
		{"@scope (main) { p { color: red } }", ""},
		{"main { content: \"\\7D\" }", ""},
		{"@important-thing {}", ""},
		// Escapes outside a string, a url and a comment.
		{"@\\69mport \"x.css\";", at("c.css", "1:1", escape)},
		{"a { b: c !\\69mportant }", at("c.css", "1:11", escape)},
		{"u\\72l(x{y) } }", at("c.css", "1:1", escape)},
		{"a { b: \\75rl(x) }", at("c.css", "1:8", escape)},
		{":\\72oot { --a: 1 }", at("c.css", "1:2", escape)},
		{":root { \\2d-a: x }", at("c.css", "1:9", escape)},
		{"a { b: c !\\69\r\nmportant }", at("c.css", "1:11", escape)},
		{"@\\69\r\nmport \"x\";", at("c.css", "1:1", escape)},
		{"a::before { content: \"\\201C\" } /* C:\\dir */ a { b: url(a\\)b.png) }", ""},
		{"a {\r\n  b: c;\r\n}\r\n}", at("c.css", "4:1", brace)},
		// Tokens a browser reads otherwise.
		{"a{b:url('x' ')')}}c{d:e}", at("c.css", "1:5", tokenTold)},
		{"a { content: \"x\n }", at("c.css", "1:14", tokenTold)},
		{"a { content: \"C:\\dir\n }", at("c.css", "1:14", tokenTold)},
		{"a { b: url(\"x\" y) }", at("c.css", "1:8", tokenTold)},
		{"a { mask: url(\"data:image/svg+xml,%3Csvg%3E\") } a { b: url( 'x' ) } a { b: url(x) } a { b: URL(\"}\") }", ""},
		// Open at the end, the innermost told: floe's } after the file would close it.
		{"main { color: red", at("c.css", "1:6", unclosed)},
		{"a{b:f(", at("c.css", "1:5", unclosed)},
		{"@media x{a{b:c}", at("c.css", "1:9", unclosed)},
		{"a{b:x} /* note", at("c.css", "1:8", unclosed)},
		{"a{b:'x", at("c.css", "1:5", unclosed)},
		{"a{b:'x\\'", at("c.css", "1:5", unclosed)},
		{"a{b:url(", at("c.css", "1:5", unclosed)},
		{"a{b:url(\"x\\\")}c{d:e}", at("c.css", "1:5", unclosed)},
		{"a { b: url(a\\ }", at("c.css", "1:8", unclosed)},
		{"a{b:'x'} /**/", ""},
		{"a { background: url(\"x.png\")", at("c.css", "1:3", unclosed)},
		{"a url(\"x\")", ""},
	} {
		if got := told(css.Check("c.css", css.Preprocess([]byte(tt.src)))); got != tt.want {
			t.Errorf("Check(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

// CheckPalette passes :root of --name and color-scheme alone, refusing each other rule and declaration and an
// important flag where it starts, and tells nothing of the shape when Check refuses the file.
func TestCheckPalette(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{":root { --a: red; color-scheme: light dark; }", ""},
		{":root { --a: { x: 1 }; }", ""},
		{":ROOT { --a: 1 }", ""},
		{":root { --a: f(}); } :root { --b: [}]; }", ""},
		{":root { --a: { x !important }; } :root { --b: red !important x; --c: \"!important\" }", ""},
		{"/* a palette */\n:root {\n  --a: red;\n}\n", ""},
		{":root { --x: red; }\nbody { color: blue }", at("p.css", "2:1", rule)},
		{":root { --x: red !important; margin: 0 }\n@media (x) { :root { --y: 1 } }", lines(
			at("p.css", "1:18", important), at("p.css", "1:30", rule), at("p.css", "2:1", rule))},
		{":root { --a: red ! important; }", at("p.css", "1:18", important)},
		{":root { --a: red !/**/important; }", at("p.css", "1:18", important)},
		{":root { color-scheme: dark !IMPORTANT; }", at("p.css", "1:28", important)},
		{":root { :root { --a: 1 } }", at("p.css", "1:9", rule)},
		{":root { color: red }", at("p.css", "1:9", rule)},
		{"html { --a: 1 }", at("p.css", "1:1", rule)},
		{":root { color-scheme { x } }", at("p.css", "1:22", rule)},
		{":root { --x: 1 } }", at("p.css", "1:18", brace)},
		{":\\72oot { --a: 1 }", at("p.css", "1:2", escape)},
		{":root{--a}\n:root{--b: red}\n:root{--c: red !important}", lines(at("p.css", "1:10", rule), at("p.css", "3:16", important))},
		{":root{color-scheme: dark {x} }\n:root{--a: red !important}", lines(at("p.css", "1:26", rule), at("p.css", "2:16", important))},
		{":root { --a; --b: red !important }", lines(at("p.css", "1:12", rule), at("p.css", "1:23", important))},
	} {
		if got := told(css.CheckPalette("p.css", css.Preprocess([]byte(tt.src)))); got != tt.want {
			t.Errorf("CheckPalette(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

func lines(s ...string) string { return strings.Join(s, "\n") }

// Preprocess makes CRLF, CR and FF a LF.
func TestPreprocess(t *testing.T) {
	t.Parallel()

	if got := string(css.Preprocess([]byte("a\r\nb\rc\fd\ne"))); got != "a\nb\nc\nd\ne" {
		t.Errorf("Preprocess = %q", got)
	}
	// An empty file stays one: nil would be no file.
	if css.Preprocess([]byte{}) == nil {
		t.Error("Preprocess of an empty file is nil")
	}
}
