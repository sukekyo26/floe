package pagediff

import (
	"strings"
	"testing"
)

var testWords = Words{Gone: "Gone", Before: "Earlier"}

// TestMark marks pairs of pages: the words in their text nodes alone, a block framed only when its words cannot show
// what differs, what is gone put in under its section's box and never in a part, the page's own ids kept.
func TestMark(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, from, to string
		side           Side
		has, hasNot    []string
	}{
		{
			name: "words", from: "<p>The count is 27 now.</p>", to: "<p>The count is 28 now.</p>", side: Alone,
			has:    []string{`<del class="floe-diff-del">7</del><ins class="floe-diff-ins">8</ins>`, `id="floe-diff.0"`},
			hasNot: []string{classChange},
		},
		{
			name: "in an element", from: "<p>a <b>bold</b> c</p>", to: "<p>a <b>bolder</b> c</p>", side: Alone,
			has: []string{`<b>bold<ins class="floe-diff-ins">er</ins></b>`},
		},
		{
			name: "no mark held", from: "<p>Write <textarea>red</textarea> here</p>", to: "<p>Write <textarea>blue</textarea> here</p>", side: Alone,
			has:    []string{`<p class="floe-diff-ch">`},
			hasNot: []string{"<ins", "<del"},
		},
		{
			name: "a row gone", from: "<h2>T</h2><table><tr><td>a</td></tr><tr><td>b</td></tr></table>", to: "<h2>T</h2><table><tr><td>a</td></tr></table>", side: Alone,
			has:    []string{`</table><div class="floe-diff-gone"><span class="floe-diff-at" id="floe-diff.0"></span><span class="floe-diff-sr">`, `<del class="floe-diff-del">b</del>`},
			hasNot: []string{"<tr><div", "</tr><div"},
		},
		{
			name: "own ids", from: `<h2 id="overview">Overview</h2><p>x</p>`, to: `<h2 id="overview">Overview and scope</h2><p>x</p>`, side: Alone,
			has: []string{`<h2 id="overview">`, `id="floe-diff.0"`},
		},
		{
			name: "details", from: `<details name="g"><p>The first one</p></details><details name="g"><p>The second one</p></details>`,
			to: `<details name="g"><p>The first one, more</p></details><details name="g"><p>The second one, more</p></details>`, side: Alone,
			has:    []string{`<details open=""><p><span class="floe-diff-at"`, `<details open=""><p>The second one`},
			hasNot: []string{`name="g"`},
		},
		{
			name: "earlier, words only added", from: "<ul><li>Second item</li></ul>", to: "<ul><li>Second item, mended</li></ul>", side: Earlier,
			has:    []string{`id="floe-diff.0"`},
			hasNot: []string{classChange, "<del", "<ins"},
		},
		{
			name: "earlier, words gone", from: "<ul><li>Second item, old</li></ul>", to: "<ul><li>Second item</li></ul>", side: Earlier,
			has:    []string{`<del class="floe-diff-del">, old</del>`, `id="floe-diff.0"`},
			hasNot: []string{classChange, "<ins"},
		},
		{
			name: "copy apart", from: `<h2>T</h2><p>kept</p><details name="g"><p>a gone part</p></details>`, to: `<h2>T</h2><p>kept</p>`, side: Alone,
			has: []string{`<details name="floe-diff.old.g" class="floe-diff-gone">`},
		},
		{
			name: "later beside, what is gone a hole", from: "<h2>T</h2><p>kept</p><p>gone</p>", to: "<h2>T</h2><p>kept</p>", side: Later,
			has:    []string{`<span class="floe-diff-hole"><span class="floe-diff-at" id="floe-diff.0"></span></span>`},
			hasNot: []string{classGone},
		},
	} {
		got, err := Mark([]byte(tc.from), []byte(tc.to), testWords, tc.side)
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
