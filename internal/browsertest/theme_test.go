package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// themeList is the list the stub answers for the colors: a favourite, a description, tags, links (one whose file is
// gone), a project of its own, and more than the smallest page size, so that the pager shows.
const themeList = `[
	{ slug: 'a', title: 'Alpha', description: 'about alpha', tags: ['plan', 'design'], project: 'p', createdAt: at(1), updatedAt: at(9), favorite: true },
	{ slug: 'b', title: 'Beta', description: '', tags: ['plan'], project: 'q', createdAt: at(1), updatedAt: at(8) },
	{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) },
	{ slug: 'm', title: 'Missing', description: '', path: '/tmp/x/m.html', missing: true, createdAt: at(1), updatedAt: at(1) },
	...Array.from({ length: 10 }, (_, i) => ({ slug: ` + "`p${i}`" + `, title: ` + "`P${i}`" + `, description: '', createdAt: at(2), updatedAt: at(2) })),
]`

// measureFields measures what measureContrast leaves out: the value of a field, its placeholder and the words of a
// select, each against the field's own ground. It returns one line for each below 4.5:1.
const measureFields = `(() => {
  const rgb = (c) => c.match(/[\d.]+/g).slice(0, 3).map(Number);
  const lum = (c) => { const [r, g, b] = rgb(c).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const low = [];
  for (const [what, e, pseudo] of [['#q', '#q', ''], ['#q::placeholder', '#q', '::placeholder'], ['#sort', '#sort', '']]) {
    const el = document.querySelector(e);
    const r = ratio(getComputedStyle(el, pseudo || null).color, getComputedStyle(el).backgroundColor);
    if (r < 4.5) low.push(what + ' ' + r.toFixed(2));
  }
  return low.join('\n');
})()`

// TestUITheme opens the management page and the frame light whatever the OS prefers, switches them dark and keeps it,
// draws them so in either without storage, leaves the page in the frame to the OS's preference, and measures that every
// text and field reads in both, the selected box's tick drawn in the color it sits on.
func TestUITheme(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")
	s.floe(t, "publish", s.page(t, "scheme.yaml", "Scheme", "<script>setInterval(() => parent.postMessage(matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light', '*'), 100)</script>"), "--slug", "scheme")
	// The OS's preference, and no motion: a row rising in is faded while it does.
	media := func(scheme string) {
		t.Helper()

		features := []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: scheme}, {Name: "prefers-reduced-motion", Value: "reduce"}}
		if _, err := chromedp.Call(b.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Features: features}); err != nil {
			t.Fatal(err)
		}
	}
	const state = "[document.documentElement.dataset.theme ?? '-', getComputedStyle(document.body).backgroundColor, " +
		"document.getElementById('theme').getAttribute('aria-pressed')].join(' ')"
	const press = "document.getElementById('theme').click()"

	out := make([]string, 0, 32)
	media("dark")
	b.navigate("/")
	out = append(out, b.eval(state))
	b.eval(press)
	out = append(out, b.eval(state+" + ' ' + localStorage.getItem('artifacts.theme')"))
	for _, path := range []string{"/", "/a/p/", "/a/none/"} {
		b.navigate(path)
		out = append(out, b.eval(state))
	}
	// The page in the frame follows the OS, whichever the frame around it is.
	frame := func() string {
		t.Helper()

		b.navigate("/a/scheme/")
		return b.until("the page's scheme", listening)
	}
	out = append(out, frame())
	b.eval("localStorage.setItem('artifacts.theme', 'light')")
	out = append(out, frame())
	media("light")
	out = append(out, frame())
	b.eval("localStorage.setItem('artifacts.theme', 'dark')")
	out = append(out, frame())

	// Every text, field and the tick, light and dark: a list with all it can show, its menus open, two rows selected, the
	// bulk delete armed and an error; the links; the edit dialog; the bulk tags dialog with a tag on all, some and none.
	for _, theme := range []string{"light", "dark"} {
		b.eval("localStorage.setItem('artifacts.theme', '" + theme + "')")
		b.navigate("/")
		b.eval(stub(themeList))
		b.eval("for (const s of [...document.querySelectorAll('#list li:not(.group) .sel')].slice(0, 2)) s.click(); " +
			"document.getElementById('bulk-delete').click(); showError('failed'); document.getElementById('q').value = ''")
		measure := func(where string) {
			t.Helper()

			if low := b.eval(measureContrast); low != "" {
				t.Errorf("%s, %s: text that cannot be read:\n%s", theme, where, low)
			}
		}
		measure("the list")
		if low := b.eval(measureFields); low != "" {
			t.Errorf("%s: fields that cannot be read:\n%s", theme, low)
		}
		out = append(out, b.eval("getComputedStyle(document.querySelector('#list .sel:checked')).backgroundImage.includes(getComputedStyle(document.documentElement).getPropertyValue('--on-ink').replace('#', '%23'))"))
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
	}

	// Storage that throws, from before the page's first script: light, and a press switches it all the same, nothing thrown.
	if _, err := chromedp.Call(b.ctx, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: `
		window.thrown = [];
		addEventListener('error', (e) => thrown.push(e.message));
		Storage.prototype.getItem = Storage.prototype.setItem = () => { throw new DOMException('denied', 'SecurityError'); };`}); err != nil {
		t.Fatal(err)
	}
	b.navigate("/")
	out = append(out, b.eval(state+" + ' ' + thrown.length"))
	b.eval(press)
	out = append(out, b.eval(state+" + ' ' + thrown.length"))

	const light, dark = "rgb(244, 243, 240)", "rgb(14, 15, 19)"
	check(t, "theme", out, strings.Join([]string{
		"- " + light + " false", "dark " + dark + " true dark",
		"dark " + dark + " true", "dark " + dark + " true", "dark " + dark + " true",
		"dark", "dark", "light", "light",
		"true", "true",
		"- " + light + " false 0", "dark " + dark + " true 0",
	}, "|"))
}
