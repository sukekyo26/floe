package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

// uses is, for each file and function of the cli's code (its name, a method's without the receiver), how many times
// each name is there: a files.X and the app's a.X, as called or as a value, and any other name standing alone (a
// function of the package called, kitOf); in a function literal too, counted to the function it is in.
func uses(t *testing.T) map[string]map[string]map[string]int {
	t.Helper()
	byFile := map[string]map[string]map[string]int{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		funcs := map[string]map[string]int{}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			got := map[string]int{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					if x, ok := n.X.(*ast.Ident); ok && (x.Name == "files" || x.Name == "a") {
						got[x.Name+"."+n.Sel.Name]++
						return false
					}
				case *ast.Ident:
					got[n.Name]++
				}
				return true
			})
			funcs[fn.Name.Name] = got
		}
		byFile[name] = funcs
	}
	return byFile
}

// A place of one's own is looked at once in a read of them: the shape that keeps it so, as the source has it. Each
// function of own.go names the files functions of the table alone, and the functions that make the places are
// called from those of the table alone, as many times: a look added, or a place made again in a read, fails here.
func TestPlaceLooks(t *testing.T) {
	t.Parallel()

	byFile := uses(t)
	own := byFile["own.go"]
	looks := map[string][]string{
		"dataDir":       {"files.NotDir", "files.Stat"},
		"compared":      {"files.Stat"},
		"overlapping":   {"files.RealAbove", "files.Stat"},
		"ownDir":        {"files.DirFS", "files.ReadDir"},
		"newOwn":        {"files.Lstat", "files.Mkdir", "files.MkdirAll", "files.RemoveAll", "files.Rename"},
		"writeSkeleton": {"files.OpenRoot"},
	}
	for fn, got := range own {
		var names []string
		for name := range got {
			if strings.HasPrefix(name, "files.") {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		if want := looks[fn]; !slices.Equal(names, want) {
			t.Errorf("own.go's %s names %v, want %v", fn, names, want)
		}
	}

	callers := map[string]map[string]int{
		"a.ownPlace":  {"ownAtOf": 1, "serverConfig": 1},
		"a.ownAtOf":   {"ownPlaces": 1, "newOwn": 1},
		"a.ownPlaces": {"kit": 1, "newOwn": 1},
		"kitOf":       {"kit": 1, "newOwn": 1},
		"a.dataDir":   {"ownPlace": 1, "storeDir": 1, "iconsDir": 1, "skillDir": 1},
	}
	for name, want := range callers {
		got := map[string]int{}
		for _, funcs := range byFile {
			for fn, names := range funcs {
				if n := names[name]; n > 0 {
					got[fn] += n
				}
			}
		}
		if !maps.Equal(got, want) {
			t.Errorf("%s is named in %v, want %v", name, got, want)
		}
	}
	// newOwn reads with the places it made (kitOf), not with places made again (kit).
	if n := own["newOwn"]["a.kit"]; n != 0 {
		t.Errorf("newOwn names a.kit %d times, want 0", n)
	}
	newOwnRefusesFirst(t)
	dataDirLooksAbove(t)
	for _, fn := range []string{"dataDir", "compared", "overlapping", "compare", "newTower", "info", "within", "newOwn"} {
		if _, ok := own[fn]; !ok {
			t.Errorf("own.go has no %s, which the table names: %v", fn, slices.Sorted(maps.Keys(own)))
		}
	}
}

// newOwnRefusesFirst checks that newOwn makes the other places (a.ownPlaces) only after its last refusal of the place
// it makes (cli_new_no_dir): a refusal there looks at no other place.
func newOwnRefusesFirst(t *testing.T) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "own.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var refused, others token.Pos
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "newOwn" {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.BasicLit:
					if n.Value == `"cli_new_no_dir"` {
						refused = n.Pos()
					}
				case *ast.SelectorExpr:
					if x, ok := n.X.(*ast.Ident); ok && x.Name == "a" && n.Sel.Name == "ownPlaces" {
						others = n.Pos()
					}
				}
				return true
			})
		}
	}
	if refused == token.NoPos || others == token.NoPos || others < refused {
		t.Errorf("newOwn makes the other places at %d, its last refusal is at %d: want them made after it", others, refused)
	}
}

// dataDirLooksAbove checks that dataDir hands files.NotDir the directory above the place its Stat failed on
// (filepath.Dir): the place itself was just looked at.
func dataDirLooksAbove(t *testing.T) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "own.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "dataDir" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSel(call.Fun, "files", "NotDir") || len(call.Args) == 0 {
				return true
			}
			arg, ok := call.Args[0].(*ast.CallExpr)
			found = ok && isSel(arg.Fun, "filepath", "Dir")
			return false
		})
	}
	if !found {
		t.Error("dataDir does not hand files.NotDir filepath.Dir of the place")
	}
}

// isSel reports whether e is pkg.name.
func isSel(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg && sel.Sel.Name == name
}
