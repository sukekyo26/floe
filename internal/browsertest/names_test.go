package browsertest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// heads are the heads of the names of the tests that open Chrome, and the head of the name of the file each is in.
var heads = []struct{ test, file string }{
	{"TestLookRule", "look_"},
	{"TestUI", "ui_"},
	{"TestRendered", "rendered_"},
}

// opensChrome tells whether fn calls newSite or newBrowser, in its own body or a function written in it.
func opensChrome(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && (id.Name == "newSite" || id.Name == "newBrowser") {
				found = true
			}
		}
		return !found
	})
	return found
}

// TestNames holds each test of this package that opens Chrome to a head of heads, in a file of the same head, as
// .claude/rules/browsertest.md says. It opens no Chrome itself.
func TestNames(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || !opensChrome(fn) {
				continue
			}
			name, held := fn.Name.Name, false
			for _, h := range heads {
				if name == h.test || h.test != "TestLookRule" && strings.HasPrefix(name, h.test) {
					held = true
					if !strings.HasPrefix(file, h.file) {
						t.Errorf("%s: %s opens Chrome in a file not named %s…: move it to one", file, name, h.file)
					}
				}
			}
			if !held {
				t.Errorf("%s: %s opens Chrome under a name of no head, TestLookRule, TestUI… or TestRendered…: rename it", file, name)
			}
		}
	}
}
