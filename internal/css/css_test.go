package css_test

import (
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
)

func at(place, text string) string { return "c.css:" + place + ": " + text }

// Check refuses every } that closes no block, @import, escape outside a string and token a browser reads
// otherwise, each where it starts, and passes what a browser reads as floe does.
func TestCheck(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"main { color: red; }\n}\n.a { x: 1 }\n", at("2:1", brace)},
		{"@import url(x.css);\nmain{}\n@media print { @import \"y.css\"; }\n", at("1:1", imports) + "\n" + at("3:16", imports)},
		{"@IMPORT \"x.css\";", at("1:1", imports)},
		{"@media print { main { } } }", at("1:27", brace)},
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
		{"@\\69mport \"x.css\";", at("1:1", escape)},
		{"a { b: c !\\69mportant }", at("1:11", escape)},
		{"u\\72l(x{y) } }", at("1:1", escape)},
		{"a { b: \\75rl(x) }", at("1:8", escape)},
		{":\\72oot { --a: 1 }", at("1:2", escape)},
		{":root { \\2d-a: x }", at("1:9", escape)},
		{"a { b: c !\\69\r\nmportant }", at("1:11", escape)},
		{"@\\69\r\nmport \"x\";", at("1:1", escape)},
		{"a::before { content: \"\\201C\" } /* C:\\dir */ a { b: url(a\\)b.png) }", ""},
		{"a {\r\n  b: c;\r\n}\r\n}", at("4:1", brace)},
		// Tokens a browser reads otherwise.
		{"a{b:url('x' ')')}}c{d:e}", at("1:5", tokenTold)},
		{"a { content: \"x\n }", at("1:14", tokenTold)},
		{"a { content: \"C:\\dir\n }", at("1:14", tokenTold)},
		{"a { b: url(\"x\" y) }", at("1:8", tokenTold)},
		{"a { mask: url(\"data:image/svg+xml,%3Csvg%3E\") } a { b: url( 'x' ) } a { b: url(x) } a { b: URL(\"}\") }", ""},
		// Open at the end, the innermost told: floe's } after the file would close it.
		{"main { color: red", at("1:6", unclosed)},
		{"a{b:f(", at("1:5", unclosed)},
		{"@media x{a{b:c}", at("1:9", unclosed)},
		{"a{b:x} /* note", at("1:8", unclosed)},
		{"a{b:'x", at("1:5", unclosed)},
		{"a{b:'x\\'", at("1:5", unclosed)},
		{"a{b:url(", at("1:5", unclosed)},
		{"a{b:url(\"x\\\")}c{d:e}", at("1:5", unclosed)},
		{"a { b: url(a\\ }", at("1:8", unclosed)},
		{"a{b:'x'} /**/", ""},
		{"a { background: url(\"x.png\")", at("1:3", unclosed)},
		{"a url(\"x\")", ""},
	} {
		if got := told(css.Check("c.css", css.Preprocess([]byte(tt.src)))); got != tt.want {
			t.Errorf("Check(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

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
