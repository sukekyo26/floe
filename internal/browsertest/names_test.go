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

// opening returns the functions and methods of decls, by name, that reach newSite or newBrowser: calling them, or
// naming them as a value, in their own bodies or through others of decls.
// LIMIT: a name is matched by its identifier alone: a method is not told by its receiver's type, and a variable of a
// function's name is taken for it, which tells more functions open Chrome, never fewer.
func opening(decls []*ast.FuncDecl) map[string]bool {
	uses := map[string]map[string]bool{}
	for _, fn := range decls {
		if fn.Body == nil {
			continue
		}
		named := uses[fn.Name.Name]
		if named == nil {
			named = map[string]bool{}
			uses[fn.Name.Name] = named
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				named[id.Name] = true
			}
			return true
		})
	}
	opens := map[string]bool{"newSite": true, "newBrowser": true}
	for grew := true; grew; {
		grew = false
		for name, named := range uses {
			if opens[name] {
				continue
			}
			for used := range named {
				if opens[used] {
					opens[name], grew = true, true
					break
				}
			}
		}
	}
	return opens
}

// isTest tells whether fn is a test: a function named Test… that takes a *testing.T.
func isTest(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || len(fn.Type.Params.List) != 1 {
		return false
	}
	star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "T"
}

// TestNames holds each test of this package that opens Chrome (it reaches newSite or newBrowser, through the
// package's functions too) to a head of heads, in a file of the same head, as .claude/rules/browsertest.md says. It
// opens no Chrome itself.
func TestNames(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var decls []*ast.FuncDecl
	in := map[*ast.FuncDecl]string{}
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				decls = append(decls, fn)
				in[fn] = file
			}
		}
	}
	opens := opening(decls)
	for _, fn := range decls {
		if !isTest(fn) || !opens[fn.Name.Name] {
			continue
		}
		name, file, held := fn.Name.Name, in[fn], false
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
