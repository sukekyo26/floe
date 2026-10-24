package server_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSendOnlyProtocol checks that send, which answers text, answers the protocol's refusals alone: a fixed
// text, never one of the catalog nor an error's. The user's mistakes and floe's failures go through refuse,
// which answers a reader that takes JSON with the report, and the rest as text with send. decodeStrict says
// what encoding/json could not read, and sendJSON cannot go through refuse, which answers with it.
func TestSendOnlyProtocol(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"refuse": true, "decodeStrict": true, "sendJSON": true}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || allowed[fn.Name.Name] {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "send" || len(call.Args) != 3 {
					return true
				}
				if lit, ok := call.Args[2].(*ast.BasicLit); !ok || lit.Kind != token.STRING {
					t.Errorf("%s: %s sends %s as text: send the protocol's refusals alone, the rest through refuse",
						fset.Position(call.Pos()), fn.Name.Name, src[fset.Position(call.Args[2].Pos()).Offset:fset.Position(call.Args[2].End()).Offset])
				}
				return true
			})
		}
	}
}
