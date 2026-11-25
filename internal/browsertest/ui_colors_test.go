package browsertest_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp/kb"
)

// colorsList is the list the stub answers for the colors: a favourite, a description, tags, links (one whose file is
// gone), a project of its own, what is new, templates, and enough rows to scroll.
const colorsList = `[
	{ slug: 'a', title: 'Alpha', description: 'about alpha', tags: ['plan', 'design'], project: 'p', news: 'version', version: 3, template: 'report', createdAt: at(1), updatedAt: at(9), favorite: true },
	{ slug: 'b', title: 'Beta', description: '', tags: ['plan'], project: 'q', news: 'new', template: 'design', createdAt: at(1), updatedAt: at(8) },
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

// measureStars measures every star drawn in the star's colour (one pressed or pointed at, the shelf's, a search row's):
// as JSON, low, a line for each below 3:1 against its ground (WCAG 1.4.11) or not of --color-star, and on, where each is
// ("row sel": a row's, on --color-sel), told by the bar or the place it is in.
const measureStars = `(() => {` + colours + `
  const root = getComputedStyle(document.documentElement);
  const named = new Map(['panel', 'hover', 'bg', 'new-soft', 'subtle', 'sel', 'ink'].map((n) => [root.getPropertyValue('--color-' + n), n]));
  const low = [], on = new Set();
  for (const svg of document.querySelectorAll('[data-testid=star] svg, [data-view=fav] svg, [data-testid=star-mark]')) {
    const button = svg.closest('[data-testid=star]');
    if (svg.getClientRects().length === 0 || (button && button.ariaPressed !== 'true' && !button.matches(':hover'))) continue;
    const at = svg.closest('#read-actions, #frame-actions, #find-actions')?.id ??
      (svg.dataset.testid === 'star-mark' ? 'star-mark' : svg.closest('[data-view=fav]') ? 'fav' : 'row');
    const s = getComputedStyle(svg), under = ground(svg.parentElement);
    const drawn = faded(s.fill === 'none' ? s.stroke : s.fill, under.opacity), shown = over(drawn, under.colour), r = ratio(shown, under.colour);
    const where = at + ' ' + (named.get(hex(under.colour)) ?? hex(under.colour));
    on.add(where);
    if (r < 3) low.push(where + ': ' + hex(shown) + ' ' + r.toFixed(2));
    if (hex(drawn) !== root.getPropertyValue('--color-star')) low.push(where + ': ' + hex(drawn) + ' is not --color-star');
  }
  return JSON.stringify({ low, on: [...on] });
})()`

// failStar presses the star sel while every write fails, and waits for the page to say so in its error.
func failStar(b *browser, sel string) {
	b.t.Helper()

	b.eval("window.failWrites = true; document.querySelector(" + js(sel) + ").click()")
	b.until("the error", "!document.getElementById('error').hidden")
	b.eval("window.failWrites = false")
}

// escapeDialog presses Escape until the modal id closes: the first may only clear the words of its search field.
func escapeDialog(b *browser, id string) {
	b.t.Helper()

	open := "document.getElementById(" + js(id) + ").open"
	b.pressKey(kb.Escape)
	if b.eval(open) == "true" {
		b.pressKey(kb.Escape)
	}
	b.until("the modal "+id+" to close", "!"+open)
}

// TestUIColors draws the management page and the frame light whatever the OS prefers, leaves the page in the frame to
// the OS's preference, and measures that every text and field reads and every star is seen, the colors copied into data URIs held.
func TestUIColors(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "scheme.yaml", "Scheme", "<script>setInterval(() => parent.postMessage(matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light', '*'), 100)</script>"), "--slug", "scheme")
	// The OS's preference, and no motion: a row rising in is faded while it does.
	media := func(medium, scheme string) {
		t.Helper()

		b.media(medium, &emulation.MediaFeature{Name: "prefers-color-scheme", Value: scheme}, &emulation.MediaFeature{Name: "prefers-reduced-motion", Value: "reduce"})
	}
	const ground = "getComputedStyle(document.body).backgroundColor"
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")
	// A real page a, for the frame beside the stubbed list to read and tell read.
	s.floe(t, "publish", s.page(t, "a.yaml", "Alpha", "<p>a</p>"), "--slug", "a")

	out := make([]string, 0, 16)
	for _, scheme := range []string{"dark", "light"} {
		// The server's list for the pages first, the list stubbed after.
		b.unstub()
		media("", scheme)
		// Light whatever the OS prefers: the list, a page's frame and a missing page's; the page in the frame reads the
		// OS's preference.
		for _, path := range []string{"/", "/a/p/", "/a/none/", "/a/scheme/"} {
			b.navigate(path)
			out = append(out, b.eval(ground))
		}
		out = append(out, b.until("the page's scheme", listening))

		// Every text and field: a list with all it can show, two rows selected, the bulk delete armed and
		// an error; the links; the edit dialog; the bulk tags dialog with a tag on all, some and none.
		b.navigate("about:blank")
		b.stub(colorsList)
		b.navigate("/")
		b.eval("for (const s of [...document.querySelectorAll('#list li[data-slug] [data-testid=sel]')].slice(0, 2)) s.click(); " +
			"document.getElementById('bulk-delete').click()")
		failStar(b, "#list li[data-slug=b] [data-testid=star]")
		// stars measures the stars, and that there is one of each want, on its ground.
		stars := func(where string, want ...string) {
			t.Helper()

			var got struct{ Low, On []string }
			if err := json.Unmarshal([]byte(b.settled(measureStars)), &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Low) > 0 {
				t.Errorf("OS %s, %s: stars that cannot be seen:\n%s", scheme, where, strings.Join(got.Low, "\n"))
			}
			for _, w := range want {
				if !slices.Contains(got.On, w) {
					t.Errorf("OS %s, %s: no star measured at %q, only at %q", scheme, where, w, got.On)
				}
			}
		}
		// starPointed is stars with the pointer on sel. The menus measureContrast opens lie over the rows: they shut first.
		starPointed := func(sel, want string) {
			t.Helper()

			b.eval("for (const d of document.querySelectorAll('details[open]')) d.open = false")
			b.eval("document.querySelector(" + js(sel) + ").scrollIntoView({ block: 'center' })")
			r := b.rect("document.querySelector(" + js(sel) + ")")
			b.point(r.X+r.Width/2, r.Y+r.Height/2)
			b.until("the pointer on "+sel, "document.querySelector("+js(sel)+").matches(':hover')")
			stars(sel+" pointed at", want)
		}
		measure := func(where string, want ...string) {
			t.Helper()

			if low := b.eval(measureContrast); low != "" {
				t.Errorf("OS %s, %s: text that cannot be read:\n%s", scheme, where, low)
			}
			stars(where, want...)
		}
		measure("the list", "row sel", "fav panel")
		// A menu shut has no items: a row's is opened for them
		menuItemShown(b, "#list li[data-slug=b]", "edit", "")
		measure("the list, a row's menu open")
		if low := b.eval(measureFields); low != "" {
			t.Errorf("OS %s: fields that cannot be read:\n%s", scheme, low)
		}
		// The data URI holds the color it copies: the tick --on-ink.
		out = append(out, b.eval("(() => { const root = getComputedStyle(document.documentElement); "+
			"const holds = (selector, color) => getComputedStyle(document.querySelector(selector)).backgroundImage.includes(root.getPropertyValue(color).replace('#', '%23')); "+
			"return holds('#list [data-testid=sel]:checked', '--on-ink'); })()"))
		b.eval("document.getElementById('bulk-tag').click()")
		measure("the tags dialog")
		// The tag on some, given to all and then to none, keeps its count: on the filled chip, then on the plain one.
		for _, mark := range []string{"all", "none"} {
			b.eval("[...document.querySelectorAll('[data-testid=tag-box]')].find((l) => l.querySelector('[data-testid=n]')).querySelector('input').click()")
			measure("the tags dialog, a tag on some given to " + mark)
		}
		b.eval("document.querySelector('#tag-dialog [data-close]').click()")
		pressMenuItem(b, "#list li[data-slug]", "edit")
		measure("the edit dialog")
		b.eval("document.querySelector('#edit-dialog [data-close]').click(); document.getElementById('bulk-clear').click()")
		// A row pointed at, a star pressed and one not.
		b.until("no row selected", "!document.querySelector('#list [data-testid=sel]:checked')")
		starPointed("#list li[data-slug=a] [data-testid=title]", "row hover")
		starPointed("#list li[data-slug=a] [data-testid=star]", "row subtle")
		starPointed("#list li[data-slug=b] [data-testid=star]", "row subtle")
		b.point(0, 0)
		b.until("the pointer off the rows", "!document.querySelector('#list li:hover')")
		// The bulk bar on the ground of the rows selected, its delete waiting or not; the window that finds, a row reached, an
		// archived one and none; the window of the keys and of the help of the screen.
		b.eval("for (const s of [...document.querySelectorAll('#list li[data-slug] [data-testid=sel]')].slice(0, 2)) s.click()")
		measure("the bulk bar")
		b.eval("document.getElementById('bulk-delete').click()")
		measure("the bulk bar, its delete waiting")
		b.eval("document.getElementById('bulk-clear').click()")
		pressFindOpen(b)
		// The first row, the favourite, is reached as it opens, its page in the foot; then the next.
		reached := func(i int) {
			t.Helper()

			b.until("the row reached", "document.querySelectorAll('#find [role=option]')["+strconv.Itoa(i)+"]?.ariaSelected === 'true'")
		}
		reached(0)
		b.pressKey(kb.ArrowDown)
		reached(1)
		stars("the window that finds, the next row reached", "star-mark panel")
		b.pressKey(kb.ArrowUp)
		reached(0)
		measure("the window that finds", "star-mark sel", "find-actions panel")
		b.eval("document.querySelector('#find-actions summary').click()")
		measure("the window that finds, its menu open")
		b.eval("document.querySelector('#find-actions [data-testid=more] > summary').click()")
		b.fill("#find-q", "nothing like it")
		measure("the window that finds, none")
		escapeDialog(b, "find")
		b.pressKey("?")
		b.until("the window of the keys", "document.getElementById('keys').open")
		measure("the window of the keys")
		escapeDialog(b, "keys")
		b.eval("document.getElementById('keys-open').click()")
		b.until("the window of the help", "document.getElementById('keys').open")
		measure("the window of the help of the screen")
		escapeDialog(b, "keys")
		// The row chosen, its version's mark on its ground, the bar over its page, and the words of a link gone beside the list. The
		// page read the first time, its mark answered as clearing what was new, says so in a toast to mark it unread again.
		b.size(1440, 900, false)
		// The stub answers every POST {"slug":"n"} and the toast shows only on {"cleared":true}: this write's answer is made here.
		b.eval(`(() => { const f = window.fetch;
			window.fetch = (url, init) => (init?.method === 'POST' && String(url) === '/api/pages/a/read' ? Promise.resolve(new Response('{"cleared":true}')) : f(url, init)); })()`)
		b.eval("document.querySelector('#list li[data-slug=a] [data-testid=title]').click()")
		b.until("the toast to mark a page unread", "document.querySelector('[data-testid=toast][data-page] button')")
		measure("the page beside the list, the toast to mark it unread", "row new-soft", "read-actions bg", "fav bg")
		// The page's toast goes as another page is shown.
		b.eval("document.querySelector('#list li[data-slug=m] [data-testid=title]').click()")
		measure("a link gone beside the list")
		// The rest of the shelf in its dialog, on a narrow screen, the row of views over the list, one pressed.
		b.size(390, 844, true)
		b.eval("document.getElementById('narrow-filter').click()")
		measure("the narrow shelf's dialog")
		b.eval("document.querySelector('#filter-dialog [data-close]').click()")
		measure("the narrow list")
		b.eval("document.querySelector('[data-view=fav]').click()")
		b.until("the favourites", "document.querySelector('[data-view=fav]').ariaPressed === 'true'")
		stars("the narrow list, the favourites pressed", "fav ink")
		b.unsize()
		// A folder open, its rows of each kind, and one of its pages in the frame under its title: the folder is the server's
		// too, which frames the page it finds.
		s.write(t, "f/index.html", "<p>f</p>")
		s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "f"), "--slug", "f", "--force")
		b.stub(colorsList[:strings.LastIndex(colorsList, "]")] + "{ slug: 'f', title: 'Folder', description: '', path: '/tmp/x/f', folder: true, createdAt: at(1), updatedAt: at(1) }]")
		b.navigate("/?kind=folder&open=f")
		b.until("the folder open", "document.querySelectorAll('#list li[data-testid=tree]').length === 3")
		measure("the folders")
		b.navigate("/a/f/?file=index.html")
		b.until("the folder's page", "!document.getElementById('frame').hidden")
		measure("a folder's page in the frame")
		// The frame's bar, its menu open.
		b.navigate("/a/a/")
		b.until("the title", "document.getElementById('frame-title').textContent")
		b.eval("document.querySelector('#frame-actions [data-testid=more] > summary').click()")
		measure("the frame's bar", "frame-actions bg")
		// What a frame prints: the note naming the page alone, with the error under a failed write, and a missing page's.
		media("print", scheme)
		failStar(b, "#frame-actions [data-testid=star]")
		measure("the frame printed")
		b.navigate("/a/none/")
		b.until("the missing page", "!document.getElementById('notfound').hidden")
		measure("a missing page's frame printed")
		media("", scheme)
	}

	const light = "rgb(244, 243, 240)"
	check(t, "colors", out, strings.Join([]string{
		light, light, light, light, "dark", "true",
		light, light, light, light, "light", "true",
	}, "|"))
}
