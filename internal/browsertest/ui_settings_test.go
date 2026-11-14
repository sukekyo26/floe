package browsertest_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"
)

// TestUISettings opens the settings from the bar's button, shown at the width of a desk, not of a phone, the bar within
// the window at both, on their list: the default theme and the task mode, each with its value. Going into one focuses
// its heading, and the way back focuses the row it came from; Escape closes the modal from any part, the focus back on
// the button. The theme's part shows a card of each theme with its sample drawn in the sandbox, and writes the default
// theme chosen from them, which a new page is then drawn in; the mode's part writes the task mode alone, which floe guide
// then reads. A settings.json that cannot be read is said on the list, and choosing mends it.
func TestUISettings(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	out := make([]string, 0, 16)
	for _, width := range []int64{1440, 390} {
		b.navigate("/")
		b.size(width, 900, width < 600)
		// Shown where the bar has room for it, hidden at 40rem or less; the bar within the window either way
		out = append(out, b.eval(`(() => {
			const r = document.getElementById('settings-open').getBoundingClientRect();
			return [r.width > 0 && r.right <= innerWidth, document.documentElement.scrollWidth <= innerWidth].join(' ');
		})()`))
	}
	b.size(1440, 900, false)
	// The parts shown, the title, the way back and the close button's word
	const seen = "[document.getElementById('settings').dataset.panel, document.getElementById('settings-title').textContent, document.getElementById('settings-back').hidden, document.querySelector('#settings [data-close]').textContent, document.getElementById('settings-list').hidden, document.getElementById('theme-grid').hidden, document.getElementById('settings-mode').hidden].join(' / ')"
	const focused = "document.activeElement.id || document.activeElement.dataset.panel"
	b.eval("document.getElementById('settings-open').focus(); document.getElementById('settings-open').click()")
	b.until("the list's values", "document.getElementById('settings-mode-now').textContent !== ''")
	out = append(out, b.eval(seen), b.eval("document.getElementById('settings-theme-now').textContent + ' / ' + document.getElementById('settings-mode-now').textContent"))
	b.eval("document.querySelector('.settings-item[data-panel=mode]').click()")
	out = append(out, b.eval(seen), b.eval(focused))
	b.eval("document.getElementById('settings-back').click()")
	out = append(out, b.eval(seen), b.eval(focused))
	b.eval("document.querySelector('.settings-item[data-panel=theme]').click()")
	b.until("the cards", "document.querySelectorAll('#theme-grid .theme-card').length >= 12")
	out = append(out, b.eval(seen), b.eval(focused),
		b.eval("document.getElementById('settings').open"),
		b.eval("document.getElementById('theme-now').textContent"),
		// default is the one now, picked; each card's sample in the sandbox, from the server
		b.eval("document.querySelector('#theme-grid input:checked').value"),
		b.eval("[...document.querySelectorAll('#theme-grid iframe')].every((f) => f.getAttribute('sandbox') === '' && f.src.endsWith('/sample'))"),
		b.eval("document.querySelector('#theme-grid input[value=ink]').closest('label').querySelector('.ds').textContent"),
	)
	b.eval("document.querySelector('#theme-grid input[value=ink]').click(); document.getElementById('theme-choose').click()")
	b.until("the toast", "[...document.querySelectorAll('.toast')].some((e) => e.textContent.includes('Changed the default theme to ink'))")
	out = append(out, b.eval("document.getElementById('settings').open"), strings.TrimSpace(s.floe(t, "theme", "default")))
	// Escape from a part closes the modal, the focus back on its button
	b.eval("document.getElementById('settings-open').focus(); document.getElementById('settings-open').click()")
	b.until("the list's values", "document.getElementById('settings-mode-now').textContent !== ''")
	b.eval("document.querySelector('.settings-item[data-panel=mode]').click()")
	b.pressKey(kb.Escape)
	b.until("closed by Escape", "!document.getElementById('settings').open")
	out = append(out, b.eval(focused))
	// The task mode on: written alone, the theme kept, and floe guide prints its Tasks section
	b.eval("document.getElementById('settings-open').click()")
	b.until("the list's values", "document.getElementById('settings-mode-now').textContent !== ''")
	b.eval("document.querySelector('.settings-item[data-panel=mode]').click()")
	out = append(out, b.eval("document.getElementById('task-mode').checked"))
	b.eval("document.getElementById('task-mode').click(); document.getElementById('mode-save').click()")
	b.until("the mode's toast", "[...document.querySelectorAll('.toast')].some((e) => e.textContent.includes('Turned the task mode on'))")
	set, err := os.ReadFile(filepath.Join(s.Env["FLOE_DIR"], "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("document.getElementById('settings').open"), strings.TrimSpace(string(set)), strconv.FormatBool(strings.Contains(s.floe(t, "guide"), "\n## Tasks\n")))
	b.eval("document.getElementById('settings-open').click()")
	b.until("the list's values", "document.getElementById('settings-mode-now').textContent === 'On'")
	b.eval("document.getElementById('settings').close()")
	// The mode's part opened before the settings are read: not checked as off, nor saved, till they are; read, it is on
	const mode = "document.getElementById('task-mode').checked + ' ' + document.getElementById('mode-save').disabled"
	b.eval(`window.realFetch ??= window.fetch; window.held = null;
		window.fetch = (u, i) => (String(u).startsWith('/api/themes') && !i?.method ? new Promise((go) => { window.held = () => go(window.realFetch(u, i)); }) : window.realFetch(u, i));
		document.getElementById('settings-open').click(); document.querySelector('.settings-item[data-panel=mode]').click()`)
	b.until("the read held", "window.held !== null")
	out = append(out, b.eval(mode))
	b.eval("window.held()")
	b.until("the mode read", "document.getElementById('task-mode').checked")
	out = append(out, b.eval(mode))
	b.eval("document.getElementById('settings').close()")
	// Not read at all: the list's values are dashes, and the mode is not to be saved
	b.eval(`window.fetch = (u, i) => (String(u).startsWith('/api/themes') && !i?.method ? Promise.reject(new TypeError('Failed to fetch')) : window.realFetch(u, i));
		document.getElementById('settings-open').click()`)
	b.until("the failure", "document.getElementById('settings-theme-now').textContent === '—'")
	out = append(out, b.eval("[document.getElementById('settings-mode-now').textContent, document.getElementById('settings-broken').textContent].join(' / ')"))
	b.eval("document.querySelector('.settings-item[data-panel=mode]').click()")
	out = append(out, b.eval(mode))
	b.eval("window.fetch = window.realFetch; document.getElementById('settings').close()")
	// A page stored now is drawn in ink.
	s.floe(t, "publish", s.page(t, "new.yaml", "New", ""))
	out = append(out, b.eval("fetch('/a/new/?raw').then((r) => r.text()).then((t) => t.includes('--accent: #2b3a67'))"))
	// settings.json broken: said, and mended by choosing.
	if err := os.WriteFile(filepath.Join(s.Env["FLOE_DIR"], "settings.json"), []byte(`{"theme": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b.eval("document.getElementById('settings-open').click()")
	b.until("the cards again", "document.getElementById('settings').open && document.querySelectorAll('#theme-grid .theme-card').length >= 12")
	out = append(out, b.eval("[document.getElementById('settings-broken').textContent, document.getElementById('settings-theme-now').textContent].join(' / ')"))
	b.eval("document.querySelector('.settings-item[data-panel=theme]').click()")
	out = append(out, b.eval("document.getElementById('theme-now').textContent"))
	b.eval("document.querySelector('#theme-grid input[value=washi]').click(); document.getElementById('theme-choose').click()")
	b.until("mended", "!document.getElementById('settings').open")
	out = append(out, strings.TrimSpace(s.floe(t, "theme", "default")))
	check(t, "settings", out, strings.Join([]string{
		"true true", "false true",
		"list / Settings / true / Close / false / true / true", "default / Off",
		"mode / Mode / false / Cancel / true / true / false", "settings-title",
		"list / Settings / true / Close / false / true / true", "mode",
		"theme / Theme / false / Cancel / true / false / true", "settings-title",
		"true",
		"Default now: default. Pages stored from now on are drawn in this theme. The pages there are now do not change.", "default", "true",
		"Ink on paper: near black, an indigo ink for emphasis, its sections ruled in the ink",
		"false", "ink",
		"settings-open",
		"false", "false", `{"theme":"ink","taskMode":true}`, "true",
		"false true", "true false",
		"— / Failed to load the themes: Failed to fetch", "false true",
		"true",
		"Failed to read the settings. Saving one writes them again / —",
		"Failed to read the settings. Choosing a theme writes them again",
		"washi",
	}, "|"))
}

// TestUIPageTheme opens a page's themes from its row's menu, none chosen first and checked, picks one, which draws the
// page in it and stays checked, the focus back on the menu; a refusal is said in the modal's words, and a theme picked
// that is gone is told.
func TestUIPageTheme(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "p.yaml", "P", ""))
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("/")
	b.until("the row", "document.querySelector('#list li[data-slug=p] .theme-item')")
	open := func() {
		t.Helper()
		b.eval("document.querySelector('#list li[data-slug=p] .more').open = true; document.querySelector('#list li[data-slug=p] .theme-item').click()")
		b.until("the cards", "document.getElementById('settings').open && document.querySelectorAll('#theme-grid .theme-card').length >= 13")
	}
	// The icon seen at the head of the modal
	const icon = "[...document.querySelectorAll('#settings .modal-head svg')].filter((e) => e.getClientRects().length).map((e) => e.getAttribute('class')).join()"
	open()
	out := make([]string, 0, 16)
	out = append(out,
		b.eval("[document.getElementById('settings-title').textContent, document.getElementById('theme-page').textContent, document.getElementById('theme-now').textContent].join(' / ')"),
		b.eval("[...document.querySelector('#theme-grid .theme-card').querySelectorAll('.theme-shot, .nm')].map((e) => e.textContent).join(' / ')"),
		b.eval("JSON.stringify(document.querySelector('#theme-grid input:checked').value)"),
		b.eval(icon),
	)
	b.eval("document.querySelector('#theme-grid input[value=ink]').click(); document.getElementById('theme-choose').click()")
	b.until("the toast", "[...document.querySelectorAll('.toast')].some((e) => e.textContent.includes(\"Changed this page's theme to ink\"))")
	b.until("listed", "items.find((a) => a.slug === 'p').pickedTheme === 'ink'")
	out = append(out,
		b.eval("document.getElementById('settings').open"),
		b.until("the focus back", "document.activeElement.matches('#list li[data-slug=p] summary')"),
		b.eval("fetch('/a/p/?raw').then((r) => r.text()).then((t) => t.includes('--accent: #2b3a67'))"),
	)
	open()
	out = append(out, b.eval("document.querySelector('#theme-grid input:checked').value"))
	// Refused: changed meanwhile, or not drawn; the modal stays open
	for _, code := range []string{"409", "400"} {
		b.eval(`window.realFetch ??= window.fetch; window.fetch = (u, i) => (i?.method === 'PUT' ? Promise.resolve(new Response('{}', { status: ` + code + ` })) : window.realFetch(u, i));
			document.querySelector('#theme-grid input[value=washi]').click(); document.getElementById('theme-choose').click()`)
		out = append(out, b.until("the refusal", "[...document.querySelectorAll('.toast')].map((e) => e.textContent).find((x) => x.startsWith(\"Failed to change this page's theme\") && x.length > 40 && !window.seen?.includes(x)) ?? ''"))
		b.eval("window.seen = [...(window.seen ?? []), " + js(out[len(out)-1]) + "]")
		out = append(out, b.eval("document.getElementById('settings').open"))
	}
	b.eval("window.fetch = window.realFetch; document.getElementById('settings').close()")
	// A theme picked that is gone
	b.eval("openThemes({ ...items.find((a) => a.slug === 'p'), pickedTheme: 'acme' })")
	b.until("the cards", "document.querySelectorAll('#theme-grid .theme-card').length >= 13")
	out = append(out, b.eval("document.getElementById('theme-now').textContent"), b.eval("String(document.querySelector('#theme-grid input:checked'))"))
	// The settings again, after a page's: of their own head
	b.eval("document.getElementById('settings').close(); openThemes()")
	b.until("the settings", "document.getElementById('settings').open")
	out = append(out, b.eval("[document.getElementById('settings-title').textContent, document.getElementById('theme-page').hidden, document.getElementById('settings-list').hidden].join(' / ')"), b.eval(icon))
	check(t, "page theme", out, strings.Join([]string{
		"This page's theme / P / Choosing draws this page alone again. The theme chosen stays when the page is updated.",
		"The page then shows in default / Remove this page's theme",
		`""`, "page-icon",
		"false", "true", "true",
		"ink",
		"Failed to change this page's theme: the page changed meanwhile. Choose again", "true",
		"Failed to change this page's theme: it was not drawn, and the page keeps its look", "true",
		"Chosen: acme (not found). Choose another theme.", "null",
		"Settings / true / false", "settings-icon",
	}, "|"))
}

// TestUIThemesFit: the themes modal, of the settings and of a page, keeps its head and its buttons in the window, wide and
// narrow, tall and low, and only its cards scroll.
func TestUIThemesFit(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "p.yaml", "P", ""))
	defer b.unsize()
	const fits = `(() => {
		const m = document.getElementById('settings').getBoundingClientRect(), grid = document.getElementById('theme-grid');
		const seen = (e) => { const r = e.getBoundingClientRect(); return r.top >= m.top && r.bottom <= m.bottom && r.top >= 0 && r.bottom <= innerHeight; };
		return [seen(document.querySelector('#settings .modal-head')), seen(document.getElementById('theme-choose')),
			seen(document.querySelector('#settings [data-close]')), grid.scrollHeight > grid.clientHeight].join(' ');
	})()`
	out := make([]string, 0, 8)
	for _, size := range [][2]int64{{1440, 900}, {1440, 700}, {390, 844}, {390, 640}} {
		b.navigate("/")
		b.size(size[0], size[1], size[0] < 600)
		b.until("the row", "items.find((a) => a.slug === 'p')")
		for _, open := range []string{"openThemes(); settingsTo('theme')", "openThemes(items.find((a) => a.slug === 'p'))"} {
			b.eval(open)
			b.until("the cards", "document.querySelectorAll('#theme-grid .theme-card').length >= 12")
			out = append(out, b.settled(fits))
			b.eval("document.getElementById('settings').close()")
		}
	}
	check(t, "themes fit", out, strings.Repeat("true true true true|", 7)+"true true true true")
}
