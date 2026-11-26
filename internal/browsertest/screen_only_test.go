package browsertest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// screenOnlySelects holds the tests that set a <select>'s value and send its change event (chooseValue): the one use of "value"
// kept for good.
var screenOnlySelects = map[string]bool{
	"TestUIDialogClose": true, "TestUIDialogHold": true, "TestUIDiffPlaces": true, "TestUIDiffRead": true, "TestUIDiffScrolls": true, "TestUIProjectPills": true, "TestUIRetyped": true, "TestUITaskAdd": true, "TestUITaskAddCategory": true, "TestUITaskCategories": true, "TestUITaskEdit": true, "TestUITaskPage": true, "TestUITaskRev": true, "TestUITaskStateGone": true, "TestUITaskUp": true, "TestUIVersionAnswers": true, "TestUIVersionGone": true, "TestUIVersionPick": true, "TestUIVersionPickAhead": true, "TestUIVersionURL": true,
}

var (
	selectorRe = regexp.MustCompile("(?:querySelector(?:All)?|closest|matches)\\(\\s*(['\"`])((?:[^'\"`\\\\]|\\\\.)*?)['\"`]")
	inBracket  = regexp.MustCompile(`\[[^\]]*\]`)
	classRe    = regexp.MustCompile(`\.([A-Za-z_][\w-]*)`)
	classState = regexp.MustCompile(`classList\.contains|\.className\b`)
	valueSet   = regexp.MustCompile(`\.value\s*=[^=]|new (?:Input)?Event\(\s*'input'`)
)

// TestScreenOnly holds the management page tests to the screen alone, as .claude/rules/browsertest.md says, so that the same tests
// check the page rebuilt: in the JS of each TestUI… test and of the package's functions it calls, no class to select by but those
// of the pages in its frames (none of the build's, web/dist/index.html), no class to read a state from, no value set. It opens no
// Chrome itself. A name of the page's script is no window's (the build is one module), so a test that calls one fails itself.
// LIMIT: a class in a selector the test builds by joining text is missed.
// LIMIT: a class is the build's when its text is anywhere in the minified build, so a short one (p-2) may be found within another
// word and refused though no element has it. Read the build's class attributes if that refuses a class of a framed page.
// LIMIT: an operation that changes the page's DOM (.remove(), .hidden =, .close(), onclick =) is not seen; a React screen may draw
// over it, so the tests that do so are looked at again as their screens switch (S422-P9, P10).
// LIMIT: the JS of the browser's and the site's methods (stub, until, …) is not read: they are the package's own, held by review.
// LIMIT: a selector handed to a Go helper (b.fill(".x", …), b.drag) is read only where it stands beside querySelector in one text.
func TestScreenOnly(t *testing.T) {
	t.Parallel()

	build, err := os.ReadFile(filepath.Join("..", "ui", "web", "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	consts := map[string][]ast.Expr{}
	topLevel := map[any]bool{}
	var tests []string
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok {
				for _, spec := range g.Specs {
					if v, ok := spec.(*ast.ValueSpec); ok {
						topLevel[v] = true
						for i, id := range v.Names {
							if i < len(v.Values) {
								consts[id.Name] = append(consts[id.Name], v.Values[i])
							}
						}
					}
				}
			}
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			topLevel[fn] = true
			funcs[fn.Name.Name] = fn
			if isTest(fn) && strings.HasPrefix(fn.Name.Name, "TestUI") {
				tests = append(tests, fn.Name.Name)
			}
		}
	}
	for _, name := range tests {
		got := screenUses(funcs, consts, topLevel, name, string(build))
		selects := screenOnlySelects[name]
		if selects && !slices.Contains(got, "value") {
			t.Errorf("%s sets no select's value: take it off screenOnlySelects", name)
		}
		for _, use := range got {
			if use != "value" || !selects {
				t.Errorf("%s uses %s: test by the screen alone (.claude/rules/browsertest.md)", name, use)
			}
		}
	}
	for name := range screenOnlySelects {
		if !slices.Contains(tests, name) {
			t.Errorf("screenOnlySelects names %s, no TestUI… test: take it off", name)
		}
	}
}

// screenUses returns, sorted, what test's JS uses of the page beyond the screen: its text and that of the package's functions,
// constants and variables it names, through others.
func screenUses(funcs map[string]*ast.FuncDecl, consts map[string][]ast.Expr, topLevel map[any]bool, test, build string) []string {
	var texts []string
	seen := map[string]bool{}
	var walk func(name string)
	walk = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		var nodes []ast.Node
		if fn := funcs[name]; fn != nil {
			nodes = append(nodes, fn.Body)
		}
		for _, v := range consts[name] {
			nodes = append(nodes, v)
		}
		for _, node := range nodes {
			ast.Inspect(node, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.BasicLit:
					if n.Kind == token.STRING {
						if s, err := strconv.Unquote(n.Value); err == nil {
							texts = append(texts, s)
						}
					}
				case *ast.Ident:
					// Called, handed on as a scenario (inEveryLanguage), or a constant's text: a name of the package's, not one
					// declared in a function (the parser resolves those)
					local := n.Obj != nil && !topLevel[n.Obj.Decl]
					if !local && !strings.HasPrefix(n.Name, "Test") {
						walk(n.Name)
					}
				}
				return true
			})
		}
	}
	walk(test)
	uses := map[string]bool{}
	for _, s := range texts {
		for _, m := range selectorRe.FindAllStringSubmatch(s, -1) {
			for _, c := range classRe.FindAllStringSubmatch(inBracket.ReplaceAllString(m[2], ""), -1) {
				if strings.Contains(build, c[1]) {
					uses["class:"+c[1]] = true
				}
			}
		}
		if classState.MatchString(s) {
			uses["classState"] = true
		}
		if valueSet.MatchString(s) {
			uses["value"] = true
		}
	}
	got := make([]string, 0, len(uses))
	for use := range uses {
		got = append(got, use)
	}
	slices.Sort(got)
	return got
}
