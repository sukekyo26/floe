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

// screenOnlyNotYet holds what each management page test not yet rewritten to the screen alone still uses (rewritten in S422-P0b):
// "name:" a variable or function of the page, "class:" a class it selects by, "classState" a class it reads a state from, "value"
// a value set or an input event made. A use not listed fails, and so does one listed but no longer used.
var screenOnlyNotYet = map[string][]string{
	"TestUIBarEnd":          {"class:bar-end", "class:bar-inner", "class:brand", "class:screens", "name:setServer"},
	"TestUIColors":          {"class:edit", "class:group", "class:n", "class:page", "class:sel", "class:tag-box", "class:title", "class:toast", "class:tree", "name:closeMenus", "name:openFind", "name:openKeys", "name:readDone", "name:showError", "value"},
	"TestUIColumnFoot":      {"class:col-foot", "class:dtoc", "class:group", "class:list-closed", "class:listcol", "class:title", "name:diffAnswer"},
	"TestUIColumns":         {"class:lhead", "class:menu", "class:path", "class:sel", "class:shelf", "class:side-show", "class:title", "class:views", "classState", "name:WIDE", "name:filter", "name:layoutColumns", "name:load", "name:picked", "name:refilter", "name:render", "name:setColumn", "name:setView", "value"},
	"TestUIDialogClose":     {"class:edit", "class:menu", "class:more", "class:tag-edit", "class:title", "class:toast", "name:items", "name:load", "name:moveAnswer", "name:takeOffTask", "value"},
	"TestUIDialogHold":      {"class:menu", "class:more", "class:settings-item", "class:task-set", "class:title", "class:toast", "name:READ_MS", "name:closeDialog", "name:holdDialog", "name:picked", "name:setShown", "value"},
	"TestUIDiff":            {"class:on", "name:diffAnswer", "name:diffPair", "name:drawn", "name:load"},
	"TestUIDiffClose":       {"classState", "name:diffAnswer"},
	"TestUIDiffContents":    {"class:dtoc-item", "class:on", "name:diffAnswer"},
	"TestUIDiffFrame":       {"name:diffAnswer"},
	"TestUIDiffNotice":      {"class:btn", "class:dnote", "class:page", "class:page-note", "class:title", "class:toast", "classState", "name:picked", "name:readsGoing"},
	"TestUIDiffPick":        {"name:diffAnswer"},
	"TestUIDiffPlaces":      {"class:diff-open", "class:keys-part", "class:title", "name:diffAnswer", "name:drawDiff", "name:openKeys", "name:picked"},
	"TestUIDiffRead":        {"name:diffAnswer", "name:setServer", "value"},
	"TestUIDiffScrolls":     {"name:diffAnswer", "name:drawDiff", "name:drawn", "name:load", "name:moveChange", "value"},
	"TestUIDiffToc":         {"class:dtoc", "name:diffAnswer"},
	"TestUIDiffURLs":        {"class:diff-open", "name:diffAnswer"},
	"TestUIDiffWays":        {"class:btn", "class:closing", "class:diff-open", "class:dtoc", "class:page", "class:title", "class:toast", "name:ORDER_KEY", "name:diffAnswer", "name:picked"},
	"TestUIFindActions":     {"class:archive", "class:armed", "class:label", "class:star", "class:toast", "classState", "name:drawn", "name:load", "name:saveEdit", "name:showError", "value"},
	"TestUIFolderSide":      {"class:dir", "classState", "name:items", "name:renderFrame"},
	"TestUIFolders":         {"class:copy", "class:folder", "class:group", "class:miss", "class:more-slot", "class:open", "class:star", "class:title", "class:tree", "classState", "name:openFind", "name:refreshTimes", "name:render", "value"},
	"TestUILoadOrder":       {"name:READ_MS", "name:api", "name:drawn", "name:load", "name:noWrites", "name:opened", "name:pressedAt", "name:render", "name:showError", "name:t", "name:tasksRead", "name:tellRead"},
	"TestUIRead":            {"class:toast", "classState", "name:readsGoing"},
	"TestUIRowAgain":        {"class:more", "class:sel", "class:title", "classState", "name:ORDER_KEY", "name:load", "name:picked", "name:selected"},
	"TestUIRowAgainPressed": {"class:title", "classState", "name:drawn", "name:items", "name:load", "name:picked"},
	"TestUIRowAgainUnread":  {"class:btn", "class:page", "class:title", "class:toast", "classState", "name:keptUnread", "name:picked"},
	"TestUIShelf":           {"class:archive", "class:chip", "class:delete", "class:edit", "class:group", "class:lhead", "class:menu", "class:more", "class:open", "class:sel", "class:star", "class:title", "class:views", "classState", "name:load", "name:render", "name:toggleProject", "value"},
	"TestUITagLine":         {"class:bar", "class:diff-open", "class:frame-actions", "class:icon-btn", "class:more", "class:name", "class:rhead", "class:sel", "class:tag-box", "class:tag-edit", "class:tag-item", "class:tagline-names", "class:tagpill", "class:toast", "classState", "name:actOn", "name:items", "name:load", "name:openPageTags", "name:pickRow", "name:picked", "name:selected", "name:taskHref", "value"},
	"TestUITags":            {"class:chip", "class:edit", "class:group", "class:mark", "class:n", "class:name", "class:sel", "class:shelf", "class:tag-box", "class:tag-item", "class:tagpill", "class:tags", "class:views", "name:render", "value"},
	"TestUIUnread":          {"class:btn", "class:page", "class:title", "class:toast", "classState", "name:picked"},
	"TestUIUnreadKept":      {"class:btn", "class:page", "class:title", "class:toast", "classState", "name:markUnread", "name:readLoaded", "name:readsGoing", "name:readsWaiting", "name:toast"},
	"TestUIUnreadPublish":   {"class:btn", "class:page", "class:title", "class:toast", "class:ver", "classState", "name:markUnread"},
	"TestUIVersionGone":     {"class:dnote", "class:edit", "class:page", "class:title", "class:toast", "class:ver-note", "classState", "name:items", "name:load", "name:picked", "name:readLoaded", "value"},
}

// screenOnlySelects holds the tests that set a <select>'s value and send its change event (chooseValue): the one use of "value"
// kept for good.
var screenOnlySelects = map[string]bool{
	"TestUITaskAdd": true, "TestUITaskEdit": true, "TestUITaskPage": true, "TestUITaskRev": true, "TestUITaskStateGone": true, "TestUITaskUp": true, "TestUIVersionAnswers": true, "TestUIVersionPick": true, "TestUIVersionPickAhead": true, "TestUIVersionURL": true,
}

var (
	// The page's names: its script's top-level declarations, two spaces in; its functions are window's too (window.load()).
	pageNameRe = regexp.MustCompile(`(?m)^  (?:async )?(?:function|let|const) ([A-Za-z_$][\w$]*)`)
	pageFuncRe = regexp.MustCompile(`(?m)^  (?:async )?function ([A-Za-z_$][\w$]*)`)
	windowRe   = regexp.MustCompile(`\bwindow\.([A-Za-z_$][\w$]*)`)
	// A name used as code: followed by what follows a name in code, not by a word as in an English description.
	wordRe = regexp.MustCompile(`(^|[^\w.$'"#/=-])([A-Za-z_$][\w$]*)\s*(\(|\.|\[|\]|\?\.|=[^=>]|===|!==|&&|\|\||;|\))`)
	// Names the test's JS gives itself: declared, or a function's parameters.
	ownNameRe  = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)|,\s*([A-Za-z_$][\w$]*)\s*=[^=>]|(?:\(|,)\s*([A-Za-z_$][\w$]*)\s*(?:,[^()]*)?\)\s*=>|([A-Za-z_$][\w$]*)\s*=>`)
	selectorRe = regexp.MustCompile("(?:querySelector(?:All)?|closest|matches)\\(\\s*(['\"`])((?:[^'\"`\\\\]|\\\\.)*?)['\"`]")
	inBracket  = regexp.MustCompile(`\[[^\]]*\]`)
	classRe    = regexp.MustCompile(`\.([A-Za-z_][\w-]*)`)
	classState = regexp.MustCompile(`classList\.contains|\.className\b`)
	valueSet   = regexp.MustCompile(`\.value\s*=[^=]|new (?:Input)?Event\(\s*'input'`)
)

// TestScreenOnly holds the management page tests to the screen alone, as .claude/rules/browsertest.md says, so that the same tests
// check the page rebuilt: in the JS of each TestUI… test and of the package's functions it calls, no name of the page's script, no
// class to select by but those of the pages in its frames (none of ui.html), no class to read a state from, no value set. It opens
// no Chrome itself.
// LIMIT: a name is seen only when code follows it; one before a comma, in ${…}, after typeof, at the end of the text, as window['…'],
// declared by destructuring (let { view } = …) or below the top level of the script is missed, and so is one given to a name (x = name(…):
// the match of x takes the space before it), and a class in a selector the test builds by joining text. A test missed fails in the PR that rebuilds its
// screen.
// LIMIT: the JS of the browser's and the site's methods (stub, until, …) is not read: they are the package's own, held by review.
// LIMIT: a selector handed to a Go helper (b.fill(".x", …), b.drag) is read only where it stands beside querySelector in one text.
// LIMIT: the names a test's JS declares (const row, (t) =>) are gathered from all its texts as one: a page's name of the same
// spelling used elsewhere in the test passes.
// LIMIT: the page's names are read from ui.html: drop that part with ui.html (S422-P11).
func TestScreenOnly(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile(filepath.Join("..", "ui", "ui.html"))
	if err != nil {
		t.Fatal(err)
	}
	pageNames, pageFuncs := map[string]bool{}, map[string]bool{}
	for _, m := range pageNameRe.FindAllSubmatch(html, -1) {
		pageNames[string(m[1])] = true
	}
	for _, m := range pageFuncRe.FindAllSubmatch(html, -1) {
		pageFuncs[string(m[1])] = true
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
		got := screenUses(funcs, consts, topLevel, name, pageNames, pageFuncs, string(html))
		want := screenOnlyNotYet[name]
		selects := screenOnlySelects[name]
		if selects && !slices.Contains(got, "value") {
			t.Errorf("%s sets no select's value: take it off screenOnlySelects", name)
		}
		for _, use := range got {
			if !slices.Contains(want, use) && (use != "value" || !selects) {
				t.Errorf("%s uses %s: test by the screen alone (.claude/rules/browsertest.md)", name, use)
			}
		}
		for _, use := range want {
			if !slices.Contains(got, use) {
				t.Errorf("%s no longer uses %s: take it off screenOnlyNotYet", name, use)
			}
		}
	}
	for name := range screenOnlySelects {
		if !slices.Contains(tests, name) {
			t.Errorf("screenOnlySelects names %s, no TestUI… test: take it off", name)
		}
	}
	for name := range screenOnlyNotYet {
		if !slices.Contains(tests, name) {
			t.Errorf("screenOnlyNotYet names %s, no TestUI… test: take it off", name)
		}
	}
}

// screenUses returns, sorted, what test's JS uses of the page beyond the screen: its text and that of the package's functions,
// constants and variables it names, through others.
func screenUses(funcs map[string]*ast.FuncDecl, consts map[string][]ast.Expr, topLevel map[any]bool, test string, pageNames, pageFuncs map[string]bool, html string) []string {
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
	own := map[string]bool{}
	for _, s := range texts {
		for _, m := range ownNameRe.FindAllStringSubmatch(s, -1) {
			for _, g := range m[1:] {
				own[g] = true
			}
		}
	}
	uses := map[string]bool{}
	for _, s := range texts {
		for _, m := range wordRe.FindAllStringSubmatch(s, -1) {
			// $ alone ends a regular expression's text, as in (\/|$): the page's $ is one called
			if pageNames[m[2]] && !own[m[2]] && (m[2] != "$" || m[3] == "(") {
				uses["name:"+m[2]] = true
			}
		}
		for _, m := range windowRe.FindAllStringSubmatch(s, -1) {
			if pageFuncs[m[1]] {
				uses["name:"+m[1]] = true
			}
		}
		for _, m := range selectorRe.FindAllStringSubmatch(s, -1) {
			for _, c := range classRe.FindAllStringSubmatch(inBracket.ReplaceAllString(m[2], ""), -1) {
				if strings.Contains(html, c[1]) {
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
