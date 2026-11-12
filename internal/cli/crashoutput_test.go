package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// debug.SetCrashOutput is called in one place, serve (internal/cli/server.go): it sets where the whole process's
// crash goes, which a server of a test or a tool that opens the log must not. runtime/debug has no function that
// reads where it goes now, so the calls are counted in the source.
func TestSetCrashOutputOnlyInServe(t *testing.T) {
	t.Parallel()

	var found []string
	fset := token.NewFileSet()
	for _, top := range []string{"..", filepath.Join("..", "..", "cmd")} {
		err := filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err //nolint:wrapcheck // the test's
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "SetCrashOutput" {
					found = append(found, filepath.ToSlash(path))
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"../cli/server.go"}; !slices.Equal(found, want) {
		t.Errorf("SetCrashOutput is called in %v, want %v only", found, want)
	}
}
