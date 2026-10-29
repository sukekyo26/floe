package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// colorsList is the list the stub answers for the colors: a favourite, a description, tags, links (one whose file is
// gone), a project of its own, and more than the smallest page size, so that the pager shows.
const colorsList = `[
	{ slug: 'a', title: 'Alpha', description: 'about alpha', tags: ['plan', 'design'], project: 'p', createdAt: at(1), updatedAt: at(9), favorite: true },
	{ slug: 'b', title: 'Beta', description: '', tags: ['plan'], project: 'q', createdAt: at(1), updatedAt: at(8) },
	{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) },
	{ slug: 'm', title: 'Missing', description: '', path: '/tmp/x/m.html', missing: true, createdAt: at(1), updatedAt: at(1) },
	...Array.from({ length: 10 }, (_, i) => ({ slug: ` + "`p${i}`" + `, title: ` + "`P${i}`" + `, description: '', createdAt: at(2), updatedAt: at(2) })),
]`

// measureFields measures what measureContrast leaves out: the value of a field and its placeholder, each against the
// field's own ground. It returns one line for each below 4.5:1.
const measureFields = `(() => {
  const rgb = (c) => c.match(/[\d.]+/g).slice(0, 3).map(Number);
  const lum = (c) => { const [r, g, b] = rgb(c).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const low = [];
  for (const [what, e, pseudo] of [['#q', '#q', ''], ['#q::placeholder', '#q', '::placeholder']]) {
    const el = document.querySelector(e);
    const r = ratio(getComputedStyle(el, pseudo || null).color, getComputedStyle(el).backgroundColor);
    if (r < 4.5) low.push(what + ' ' + r.toFixed(2));
  }
  return low.join('\n');
})()`

// TestUIColors draws the management page and the frame light whatever the OS prefers, leaves the page in the frame to
// the OS's preference, and measures that every text and field reads, the colors copied into data URIs held.
func TestUIColors(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "scheme.yaml", "Scheme", "<script>setInterval(() => parent.postMessage(matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light', '*'), 100)</script>"), "--slug", "scheme")
	// The OS's preference, and no motion: a row rising in is faded while it does.
	media := func(scheme string) {
		t.Helper()

		features := []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: scheme}, {Name: "prefers-reduced-motion", Value: "reduce"}}
		if _, err := chromedp.Call(b.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Features: features}); err != nil {
			t.Fatal(err)
		}
	}
	const ground = "getComputedStyle(document.body).backgroundColor"
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")

	out := make([]string, 0, 16)
	for _, scheme := range []string{"dark", "light"} {
		// The server's list for the pages first, the list stubbed after.
		b.unstub()
		media(scheme)
		// Light whatever the OS prefers: the list, a page's frame and a missing page's; the page in the frame reads the
		// OS's preference.
		for _, path := range []string{"/", "/a/p/", "/a/none/", "/a/scheme/"} {
			b.navigate(path)
			out = append(out, b.eval(ground))
		}
		out = append(out, b.until("the page's scheme", listening))

		// Every text and field: a list with all it can show, its menus open, two rows selected, the bulk delete armed and
		// an error; the links; the edit dialog; the bulk tags dialog with a tag on all, some and none.
		b.navigate("about:blank")
		b.stub(colorsList)
		b.navigate("/")
		b.eval("for (const s of [...document.querySelectorAll('#list li:not(.group) .sel')].slice(0, 2)) s.click(); " +
			"document.getElementById('bulk-delete').click(); showError('failed'); document.getElementById('q').value = ''")
		measure := func(where string) {
			t.Helper()

			if low := b.eval(measureContrast); low != "" {
				t.Errorf("OS %s, %s: text that cannot be read:\n%s", scheme, where, low)
			}
		}
		measure("the list")
		if low := b.eval(measureFields); low != "" {
			t.Errorf("OS %s: fields that cannot be read:\n%s", scheme, low)
		}
		// The data URI holds the color it copies: the tick --on-ink.
		out = append(out, b.eval("(() => { const root = getComputedStyle(document.documentElement); "+
			"const holds = (selector, color) => getComputedStyle(document.querySelector(selector)).backgroundImage.includes(root.getPropertyValue(color).replace('#', '%23')); "+
			"return holds('#list .sel:checked', '--on-ink'); })()"))
		b.eval("document.getElementById('bulk-tag').click()")
		measure("the tags dialog")
		// The tag on some, given to all and then to none, keeps its count: on the filled chip, then on the plain one.
		for _, mark := range []string{"all", "none"} {
			b.eval("[...document.querySelectorAll('.tag-box')].find((l) => l.querySelector('.n')).querySelector('input').click()")
			measure("the tags dialog, a tag on some given to " + mark)
		}
		b.eval("document.querySelector('#tag-dialog [data-close]').click(); document.querySelector('#list li:not(.group) .edit').click()")
		measure("the edit dialog")
		b.eval("document.querySelector('#edit-dialog [data-close]').click(); document.querySelector('.tabs [data-tab=links]').click()")
		measure("the links")
		// The sort's orders, open, one of them reached and the one now marked.
		b.eval("document.getElementById('sort').click()")
		b.until("the orders", "document.getElementById('sort-menu').matches(':popover-open')")
		b.eval("document.getElementById('sort-list').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))")
		measure("the sort's orders")
		b.eval("document.getElementById('sort-menu').hidePopover()")
		// The frame's pages to pick from, open, one of them reached and the one open now marked.
		b.navigate("/a/a/")
		b.until("the title", "!document.getElementById('frame-pick').hidden")
		b.eval("document.getElementById('frame-pick').click()")
		b.until("the pages to pick from", "document.getElementById('page-pick').matches(':popover-open')")
		b.eval("document.getElementById('page-pick-q').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))")
		measure("the frame's pages to pick from")
	}

	const light = "rgb(244, 243, 240)"
	check(t, "colors", out, strings.Join([]string{
		light, light, light, light, "dark", "true",
		light, light, light, light, "light", "true",
	}, "|"))
}
