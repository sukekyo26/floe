package pagediff

import (
	"fmt"
	"strings"
	"testing"
)

var testWords = Words{Gone: "Gone", Before: "Earlier"}

// table is a table under a heading, of the head row head and the rows rows.
func table(head string, rows ...string) string {
	return "<h2>T</h2><table><thead><tr>" + head + "</tr></thead><tbody><tr>" + strings.Join(rows, "</tr><tr>") + "</tr></tbody></table>"
}

// twoTabs is two sections of floe's tabs, the second tab of each holding a kept paragraph after one of gone.
func twoTabs(gone, other string) string {
	tab := func(heading, gone string) string {
		if gone != "" {
			gone = "<p>" + gone + "</p>"
		}
		return "<h2>" + heading + `</h2><form data-floe-part="tabs"><label><input type="radio" name="tab" checked>One</label><div><p>The first tab</p></div>` +
			`<label><input type="radio" name="tab">Two</label><div>` + gone + "<p>kept</p></div></form>"
	}
	return tab("T", gone) + tab("U", other)
}

// threeTabs is floe's part tabs of three, the first picked, with a middle tab titled title holding panel; none for "".
func threeTabs(title, panel string) string {
	tab := func(title, panel, checked string) string {
		return `<label><input type="radio" name="tab"` + checked + `>` + title + `</label><div><p>` + panel + `</p></div>`
	}
	middle := ""
	if title != "" {
		middle = tab(title, panel, "")
	}
	return `<form data-floe-part="tabs">` + tab("One", "First panel", " checked") + middle + tab("Three", "Third panel", "") + `</form>`
}

// spanning is a small table whose cell spans two columns.
const spanning = `<table><tbody><tr><td colspan="2">in</td></tr></tbody></table>`

// tabs is floe's part tabs, the second not picked holding %s.
const tabs = `<form data-floe-part="tabs"><label><input type="radio" name="tab" checked>One</label><div><p>The first tab</p></div>` +
	`<label><input type="radio" name="tab">Two</label><div><p>%s</p></div></form>`

// TestMark marks pairs of pages: the words in their text nodes alone, a block framed only when its words cannot show
// what differs, what is gone put in under its section's box and never in a part, the page's own ids kept.
func TestMark(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, from, to string
		has, hasNot    []string
	}{
		{
			name: "words", from: "<p>The count is 27 now.</p>", to: "<p>The count is 28 now.</p>",
			has:    []string{`<del class="floe-diff-del">7</del><ins class="floe-diff-ins">8</ins>`, `id="floe-diff.0"`},
			hasNot: []string{classChange},
		},
		{
			name: "in an element", from: "<p>a <b>bold</b> c</p>", to: "<p>a <b>bolder</b> c</p>",
			has: []string{`<b>bold<ins class="floe-diff-ins">er</ins></b>`},
		},
		{
			name: "no mark held", from: "<p>Write <textarea>red</textarea> here</p>", to: "<p>Write <textarea>blue</textarea> here</p>",
			has:    []string{`<p class="floe-diff-ch">`},
			hasNot: []string{"<ins", "<del"},
		},
		{
			name: "two lines of one text changed", from: "<pre>alpha one\nbeta two\ngamma three\ndelta four</pre>", to: "<pre>alpha won\nbeta two\ngamma tree\ndelta four</pre>",
			has: []string{"alpha ", "gamma ", `<ins class="floe-diff-ins">w</ins>`, `<del class="floe-diff-del">h</del>`},
		},
		{
			name: "a column gone", from: table("<th>Name</th><th>Short</th><th>Long</th>", "<td>alpha</td><td>x</td><td>the long words of one</td>", "<td>beta</td><td>yy</td><td>the long words of two</td>"),
			to: table("<th>Name</th><th>Long</th>", "<td>alpha</td><td>the long words of one</td>", "<td>beta</td><td>the long words of two</td>"),
			has: []string{
				`<th class="floe-diff-gone"><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span>Short</del></th><th>Long</th>`,
				`<td>alpha</td><td class="floe-diff-gone"><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span>x</del></td>`,
				`<td>beta</td><td class="floe-diff-gone"><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span>yy</del></td>`,
			},
		},
		{
			name: "a column added", from: table("<th>Name</th><th>Long</th>", "<td>alpha</td><td>the long words of one</td>"),
			to:     table("<th>Name</th><th>New</th><th>Long</th>", "<td>alpha</td><td>added</td><td>the long words of one</td>"),
			has:    []string{`<th><ins class="floe-diff-ins">New</ins></th>`, `<td><ins class="floe-diff-ins">added</ins></td>`},
			hasNot: []string{classChange},
		},
		{
			name: "a row and a column gone", from: table("<th>Name</th><th>N</th><th>Long</th>", "<td>alpha</td><td>1</td><td>the long words of one</td>", "<td>beta</td><td>2</td><td>gone row words</td>", "<td>gamma</td><td>3</td><td>the long words of three</td>"),
			to: table("<th>Name</th><th>Long</th>", "<td>alpha</td><td>the long words of one</td>", "<td>gamma</td><td>the long words of three</td>"),
			has: []string{
				`<td>alpha</td><td class="floe-diff-gone"><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span>1</del></td><td>the long words of one</td></tr><tr class="floe-diff-gone"><td><del`,
				`<td><del class="floe-diff-del">2</del></td><td><del class="floe-diff-del">gone row words</del></td></tr><tr><td>gamma</td>`,
			},
		},
		{
			name: "the head's row gone", from: table("<th>Name</th><th>Long</th>", "<td>alpha</td><td>the long words of one</td>"),
			to:  "<h2>T</h2><table><tbody><tr><td>alpha</td><td>the long words of one</td></tr></tbody></table>",
			has: []string{`<tbody><tr class="floe-diff-gone"><th><del`, `Name</del></th><th><del class="floe-diff-del">Long</del></th></tr><tr><td>alpha</td>`},
		},
		{
			name: "a body row gone, no body left", from: table("<th>Name</th><th>Long</th>", "<td>alpha</td><td>the long words of one</td>"),
			to:  "<h2>T</h2><table><thead><tr><th>Name</th><th>Long</th></tr></thead></table>",
			has: []string{`</table><table class="floe-diff-gone">`, "alpha"},
		},
		{
			name: "a table of spans", from: table(`<th colspan="2">Both</th>`, "<td>a</td><td>x</td>", "<td>b</td><td>the long words</td>"),
			to:     table(`<th colspan="2">Both</th>`, "<td>a</td><td>x</td>"),
			has:    []string{`</table><table class="floe-diff-gone">`},
			hasNot: []string{"</td></tr><tr><td><del"},
		},
		{
			name: "a row gone", from: "<h2>T</h2><table><tr><td>a</td></tr><tr><td>b</td></tr></table>", to: "<h2>T</h2><table><tr><td>a</td></tr></table>",
			has:    []string{`<tbody><tr><td>a</td></tr><tr class="floe-diff-gone"><td><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span><span class="floe-diff-at" id="floe-diff.0"></span>b</del></td></tr></tbody></table>`},
			hasNot: []string{"</table><table"},
		},
		{
			name: "own ids", from: `<h2 id="overview">Overview</h2><p>x</p>`, to: `<h2 id="overview">Overview and scope</h2><p>x</p>`,
			has: []string{`<h2 id="overview">`, `id="floe-diff.0"`},
		},
		{
			name: "details", from: `<details name="g"><p>The first one</p></details><details name="g"><p>The second one</p></details>`,
			to:     `<details name="g"><p>The first one, more</p></details><details name="g"><p>The second one, more</p></details>`,
			has:    []string{`<details open=""><p><span class="floe-diff-at"`, `<details open=""><p>The second one`},
			hasNot: []string{`name="g"`},
		},
		{
			name: "a tab not picked", from: fmt.Sprintf(tabs, "Words of the second tab, old"), to: fmt.Sprintf(tabs, "Words of the second tab"),
			has: []string{`<label class="floe-diff-ch"><input type="radio" name="tab"/><span class="floe-diff-at" id="floe-diff.0"></span>Two</label>`},
		},
		{
			name: "a diagram gone", from: `<h2>T</h2><p>kept</p><div data-floe-part="diagram"><svg viewBox="0 0 10 10"><text>x</text></svg></div>`, to: `<h2>T</h2><p>kept</p>`,
			has: []string{`<div data-floe-part="diagram" class="floe-diff-gone floe-diff-out"><svg`},
		},
		{
			name: "items gone one after another", from: "<h2>T</h2><ul><li>one</li><li>gone item</li><li>gone too</li></ul>", to: "<h2>T</h2><ul><li>one</li></ul>",
			has:    []string{`<ul class="floe-diff-gone"><li><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span><span class="floe-diff-at" id="floe-diff.0"></span>gone item</del></li><li><del class="floe-diff-del">gone too</del></li></ul>`},
			hasNot: []string{`<ul class="floe-diff-gone"><li>one`, `id="floe-diff.1"`},
		},
		{
			name: "a tab gone, its title too", from: "<h2>T</h2><p>kept</p>" + threeTabs("Gonetitle", "Gone panel"),
			to:  "<h2>T</h2><p>kept</p>" + threeTabs("", ""),
			has: []string{`Gonetitle</del></label><div><p><del class="floe-diff-del">Gone panel</del>`},
		},
		{
			name: "tabs gone whole", from: "<h2>T</h2><p>kept</p>" + threeTabs("Gonetitle", "Gone panel"), to: "<h2>T</h2><p>kept</p>",
			has: []string{`Gonetitle</del></label><div><p><del class="floe-diff-del">Gone panel</del>`},
		},
		{
			name: "from two tabs", from: twoTabs("gone in one", "gone in other"), to: twoTabs("", ""),
			has: []string{
				`<input type="radio" name="floe-diff.old.0.tab" checked=""/>`,
				`<input type="radio" name="floe-diff.old.1.tab" checked=""/>`,
				`Two</label><div><p><del class="floe-diff-del">gone in one</del></p></div></form>`, `Two</label><div><p><del class="floe-diff-del">gone in other</del></p></div></form>`,
			},
		},
		{
			name: "an ol's third gone", from: "<h2>T</h2><ol><li>a</li><li>b</li><li>c c c</li></ol>", to: "<h2>T</h2><ol><li>a</li><li>b</li></ol>",
			has: []string{`<ol class="floe-diff-gone"><li value="3">`},
		},
		{
			name: "an ol of start 5, its third and fourth gone", from: `<h2>T</h2><ol start="5"><li>a</li><li>b</li><li>c c c</li><li>d d d</li></ol>`, to: `<h2>T</h2><ol start="5"><li>a</li><li>b</li></ol>`,
			has: []string{`<li value="7">`, `<li value="8">`},
		},
		{
			name: "a reversed ol's second gone", from: "<h2>T</h2><ol reversed><li>a a a</li><li>b b b</li><li>c c c</li></ol>", to: "<h2>T</h2><ol reversed><li>a a a</li><li>c c c</li></ol>",
			has:    []string{`<ol class="floe-diff-gone"><li value="2">`, `<ol reversed=""><li>a a a`},
			hasNot: []string{`<ol reversed="" class="floe-diff-gone">`},
		},
		{
			name: "after an item of a value", from: `<h2>T</h2><ol><li value="10">a</li><li>b b b</li></ol>`, to: `<h2>T</h2><ol><li value="10">a</li></ol>`,
			has: []string{`<li value="11">`},
		},
		{
			name: "the styles of a part only the earlier draws", from: `<head><style>.only-before{}</style><style>.both{}</style></head><h2>T</h2><p>kept</p><div data-floe-part="x"><p>gone part</p></div>`,
			to:  `<head><style>.both{}</style></head><h2>T</h2><p>kept</p>`,
			has: []string{`<style>.only-before{}</style>`},
		},
		{
			name: "words of the part itself gone", from: "<h2>T</h2><div>before<p>between</p>gone after</div>", to: "<h2>T</h2><div>before<p>between</p></div>",
			has: []string{`<div class="floe-diff-gone">`, `<del class="floe-diff-del">gone after</del></div>`},
		},
		{
			name: "gone after a header, out of it", from: `<header><h1>Title</h1><p class="status">Accepted</p></header><div class="superseded-by"><p>By the next one</p></div><h2>Context</h2><p>kept</p>`,
			to:     `<header><h1>Title</h1><p class="status">Accepted</p></header><h2>Context</h2><p>kept</p>`,
			has:    []string{`Accepted</p></header><div class="superseded-by floe-diff-gone"`},
			hasNot: []string{`Accepted</p><div`},
		},
		{
			name: "a section gone after a subsection, out of it", from: `<section><h2>A</h2><p>a kept</p><section class="sub"><h3>A1</h3><p>sub kept</p></section></section><section><h2>B</h2><p>b gone words</p></section>`,
			to:     `<section><h2>A</h2><p>a kept</p><section class="sub"><h3>A1</h3><p>sub kept</p></section></section>`,
			has:    []string{`sub kept</p></section><section class="floe-diff-gone"`},
			hasNot: []string{`sub kept</p><h2`, `sub kept</p></section><h2`},
		},
		{
			name: "gone of a page the later wraps in main", from: `<h2>T</h2><p>a</p><p>gone words long here</p><h2>U</h2><p>u</p>`,
			to:  `<main><h2>T</h2><p>a</p><h2>U</h2><p>u</p></main>`,
			has: []string{`<p>a</p><p class="floe-diff-gone">`},
		},
		{
			name: "gone of a page the later unwraps from divs", from: `<div><h2>T</h2><p>a</p><p>gone words long here</p></div><div><h2>U</h2><p>u</p></div>`,
			to:  `<h2>T</h2><p>a</p><h2>U</h2><p>u</p>`,
			has: []string{`<p>a</p><p class="floe-diff-gone">`},
		},
		{
			name: "a tab picked keeps a box checked", from: "<h2>T</h2><p>kept</p>" + fmt.Sprintf(tabs, `gone <input type="checkbox" checked> box`), to: "<h2>T</h2><p>kept</p>",
			has: []string{`<input type="checkbox" checked=""/>`},
		},
		{
			name: "a column gone by a table in a cell that spans", from: table("<th>Name</th><th>Short</th><th>Long</th>", "<td>alpha</td><td>x</td><td>the long words of one</td>", `<td>beta</td><td>yy</td><td>the long words of two `+spanning+`</td>`),
			to:  table("<th>Name</th><th>Long</th>", "<td>alpha</td><td>the long words of one</td>", `<td>beta</td><td>the long words of two `+spanning+`</td>`),
			has: []string{`<td>alpha</td><td class="floe-diff-gone"><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span>x</del></td>`},
		},
		{
			name: "copy apart", from: `<h2>T</h2><p>kept</p><details name="g"><p>a gone part</p></details>`, to: `<h2>T</h2><p>kept</p>`,
			has:    []string{`<details open="" class="floe-diff-gone"><p><del class="floe-diff-del"><span class="floe-diff-sr">Gone </span>`},
			hasNot: []string{classOut},
		},
	} {
		_, got, err := Diff([]byte(tc.from), []byte(tc.to), testWords)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		// The page after the style of the marks, which names every class.
		_, page, _ := strings.Cut(string(got), "</style>")
		for _, want := range tc.has {
			if !strings.Contains(page, want) {
				t.Errorf("%s: no %s in\n%s", tc.name, want, page)
			}
		}
		for _, not := range tc.hasNot {
			if strings.Contains(page, not) {
				t.Errorf("%s: %s in\n%s", tc.name, not, page)
			}
		}
	}
}
