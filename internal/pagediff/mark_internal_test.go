package pagediff

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var testWords = Words{Gone: "Gone", Before: "Earlier"}

// tabs is floe's part tabs, the second not picked holding %s.
const tabs = `<form data-floe-part="tabs"><label><input type="radio" name="tab" checked>One</label><div><p>The first tab</p></div>` +
	`<label><input type="radio" name="tab">Two</label><div><p>%s</p></div></form>`

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
			name: "earlier, a tab not picked", from: fmt.Sprintf(tabs, "Words of the second tab, old"), to: fmt.Sprintf(tabs, "Words of the second tab"), side: Earlier,
			has: []string{`<label class="floe-diff-ch"><input type="radio" name="tab"/><span class="floe-diff-at" id="floe-diff.0"></span>Two</label>`},
		},
		{
			name: "later beside, a tab not picked", from: fmt.Sprintf(tabs, "Words of the second tab, old"), to: fmt.Sprintf(tabs, "Words of the second tab"), side: Later,
			has: []string{`<label class="floe-diff-ch"><input type="radio" name="tab"/><span class="floe-diff-at" id="floe-diff.0"></span>Two</label>`},
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

// TestMarkSides marks the two pages of a pair side by side, by the one walk: what each shows of a place comes in pairs,
// the same places to go to, the same tab titles framed and the same details opened on both.
func TestMarkSides(t *testing.T) {
	t.Parallel()

	ids := regexp.MustCompile(`id="floe-diff\.\d+"`)
	for _, tc := range []struct{ name, from, to string }{
		{"words", "<h2>A</h2><p>The count is 27 now.</p><p>Kept.</p><p>Other words, old.</p>", "<h2>A</h2><p>The count is 28 now.</p><p>Kept.</p><p>Other words.</p>"},
		{"added and gone", "<h2>A</h2><p>kept one</p><p>only before</p><p>kept two</p>", "<h2>A</h2><p>kept one</p><p>kept two</p><p>only after</p>"},
		{"a tab not picked", fmt.Sprintf(tabs, "Words of the second tab, old"), fmt.Sprintf(tabs, "Words of the second tab")},
		{"details", `<details name="g"><p>The first one</p></details><p>Kept.</p><details name="g"><p>The second one</p></details>`,
			`<details name="g"><p>The first one, more</p></details><p>Kept.</p><details name="g"><p>The second one, more</p></details>`},
		{"sections", "<h2>A</h2><p>a</p><h2>Gone</h2><p>g</p><h2>B</h2><p>b old</p>", "<h2>A</h2><p>a</p><h2>B</h2><p>b</p><h2>New</h2><p>n</p>"},
	} {
		var told [2]string
		for i, side := range []Side{Earlier, Later} {
			got, err := Mark([]byte(tc.from), []byte(tc.to), testWords, side)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			_, page, _ := strings.Cut(string(got), "</style>")
			told[i] = fmt.Sprintf("%v labels %d details %d", ids.FindAllString(page, -1),
				strings.Count(page, `<label class="floe-diff-ch">`), strings.Count(page, `<details open="">`))
		}
		if told[0] != told[1] {
			t.Errorf("%s: the earlier shows %s, the later %s", tc.name, told[0], told[1])
		}
	}
}
