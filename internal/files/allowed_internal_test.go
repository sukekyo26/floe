package files

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// noPath are files' exported functions that call the system on no path: they look at names or at what is open, or
// take a path under an *os.Root, which keeps it under its directory (ReadIn), or look at a path through Stat, an
// entry (NotDir).
var noPath = []string{ //nolint:gochecknoglobals // a test's table
	"Hidden", "IsDataFile", "IsNotFile", "IsPage", "Lock", "NotDir", "ReadIn", "Slug", "SlugFromFile", "ValidName",
}

// makers are where a checked may be made: allowed looks at the path first (or Unix gives it as it is), and a dir or
// a name files makes in one is checked as it is.
var makers = []string{ //nolint:gochecknoglobals // a test's table
	"allowed", "allowedWritten", "walk", "full", "dir", "join", "result", "follow",
}

// callers are where the system is called on a path in files: the wrappers that take a checked, allowed's walk, and
// the files of each system, whose functions take a checked (osPaired).
var callers = []string{ //nolint:gochecknoglobals // a test's table
	"statAt", "lstatAt", "readDirAt", "openRootAt", "mkdirAllAt", "mkdirAt", "openAt", "removeAt", "removeAllAt",
	"mkdirTempAt", "readlinkAt", "next",
}

// osPaired are the functions of the files of each system that call the system on a path: each takes a checked.
var osPaired = []string{"stampAt", "statOf", "openAttrs", "realOf", "longNameOf", "openShared", "renameIn"} //nolint:gochecknoglobals // as above

// pathCalls are the calls on a path that the system makes.
var pathCalls = map[string]bool{ //nolint:gochecknoglobals // as above
	"os.Stat": true, "os.Lstat": true, "os.Open": true, "os.OpenFile": true, "os.ReadDir": true, "os.ReadFile": true,
	"os.Readlink": true, "os.OpenRoot": true, "os.OpenInRoot": true, "os.Mkdir": true, "os.MkdirAll": true,
	"os.MkdirTemp": true, "os.Rename": true, "os.Remove": true, "os.RemoveAll": true, "os.Create": true,
	"os.CreateTemp": true, "os.WriteFile": true, "os.Chmod": true, "os.Chtimes": true, "os.Symlink": true,
	"filepath.EvalSymlinks": true, "filepath.WalkDir": true, "filepath.Walk": true, "filepath.Glob": true,
}

// parsed is files' source files (no tests), by name.
func parsed(t *testing.T) map[string]*ast.File {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	got := map[string]*ast.File{}
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		got[name] = f
	}
	return got
}

// eachFunc runs do on each function declaration of the files of src, with its file's name.
func eachFunc(src map[string]*ast.File, do func(file string, fn *ast.FuncDecl)) {
	for name, f := range src {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				do(name, fn)
			}
		}
	}
}

// Every exported function of files that calls the system on a path is an entry TestAllowedEntries runs (Windows),
// or calls it on none; checked is made only where allowed looked (or of what was), and the system is called on a
// path only by the wrappers, allowed's walk and the files of each system, whose functions take a checked. So a new
// entry that forgets allowed fails here, and one that gives the system the path as given does not compile.
func TestEveryEntryAllowed(t *testing.T) {
	t.Parallel()

	src := parsed(t)
	entries := entryNames(t, src["allowed_windows_test.go"])
	eachFunc(src, func(file string, fn *ast.FuncDecl) {
		name := fn.Name.Name
		if fn.Recv == nil && ast.IsExported(name) && !slices.Contains(entries, name) && !slices.Contains(noPath, name) {
			t.Errorf("%s: %s is neither an entry of TestAllowedEntries nor of noPath", file, name)
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if id, ok := x.Type.(*ast.Ident); ok && id.Name == "checked" && !slices.Contains(makers, name) {
					t.Errorf("%s: %s makes a checked; only %v may", file, name, makers)
				}
			case *ast.CallExpr:
				call := selector(x.Fun)
				paired := file == "files_unix.go" || file == "files_windows.go"
				if pathCalls[call] && !slices.Contains(callers, name) && !paired {
					t.Errorf("%s: %s calls %s on a path; only %v may", file, name, call, callers)
				}
			}
			return true
		})
		if (file == "files_unix.go" || file == "files_windows.go") && slices.Contains(osPaired, name) {
			if p := fn.Type.Params.List; len(p) == 0 || selector(p[0].Type) != "checked" {
				t.Errorf("%s: %s does not take a checked first", file, name)
			}
		}
	})
}

// entryNames are the names of the entries TestAllowedEntries runs, as entries() of f lists them.
func entryNames(t *testing.T, f *ast.File) []string {
	t.Helper()
	if f == nil {
		t.Fatal("no allowed_windows_test.go")
	}
	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		list, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if at, isArray := list.Type.(*ast.ArrayType); !isArray || selector(at.Elt) != "entry" {
			return true
		}
		for _, e := range list.Elts {
			lit, ok := e.(*ast.CompositeLit)
			if !ok || len(lit.Elts) == 0 {
				continue
			}
			if s, ok := lit.Elts[0].(*ast.BasicLit); ok {
				if name, err := strconv.Unquote(s.Value); err == nil {
					names = append(names, name)
				}
			}
		}
		return true
	})
	return names
}

// selector is e as written, pkg.Name or Name, "" for any other expression.
func selector(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name + "." + x.Sel.Name
		}
	}
	return ""
}

// A network drive and one the system cannot tell are refused; a fixed, removable or missing one is not.
func TestRefusedDrive(t *testing.T) {
	t.Parallel()

	for kind, want := range map[uint32]bool{0: true, 1: false, 2: false, 3: false, 4: true, 5: false, 6: false} {
		if got := refusedDrive(kind); got != want {
			t.Errorf("refusedDrive(%d) = %v, want %v", kind, got, want)
		}
	}
}
