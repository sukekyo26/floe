package pagediff

import (
	"fmt"
	"strings"
	"testing"
)

var testWords = Words{Gone: "Gone", Before: "Earlier"}

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
			name: "a row gone", from: "<h2>T</h2><table><tr><td>a</td></tr><tr><td>b</td></tr></table>", to: "<h2>T</h2><table><tr><td>a</td></tr></table>",
			has:    []string{`</table><table class="floe-diff-gone"><tbody><tr><td><span class="floe-diff-sr">Gone </span><span class="floe-diff-at" id="floe-diff.0"></span>b</td></tr></tbody></table>`},
			hasNot: []string{"<td>a</td></tr></tbody></table></table>", `class="floe-diff-gone"><tbody><tr><td>a`},
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
			has:    []string{`<ul class="floe-diff-gone"><li><span class="floe-diff-sr">Gone </span><span class="floe-diff-at" id="floe-diff.0"></span>gone item</li><li>gone too</li></ul>`},
			hasNot: []string{`<ul class="floe-diff-gone"><li>one`, `id="floe-diff.1"`},
		},
		{
			name: "from two tabs", from: twoTabs("gone in one", "gone in other"), to: twoTabs("", ""),
			has: []string{
				`<input type="radio" name="floe-diff.old.0.tab" checked=""/>`,
				`<input type="radio" name="floe-diff.old.1.tab" checked=""/>`,
				"Two</label><div><p>gone in one</p></div></form>", "Two</label><div><p>gone in other</p></div></form>",
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
			has: []string{`<div class="floe-diff-gone">`, "gone after</div>"},
		},
		{
			name: "copy apart", from: `<h2>T</h2><p>kept</p><details name="g"><p>a gone part</p></details>`, to: `<h2>T</h2><p>kept</p>`,
			has:    []string{`<details class="floe-diff-gone" open=""><p><span class="floe-diff-sr">Gone </span>`},
			hasNot: []string{classOut},
		},
	} {
		got, err := Mark([]byte(tc.from), []byte(tc.to), testWords)
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
