package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// TestUIFolderSide opens a folder's page with the folder's tree on the frame's left: open where the page is, its width
// changed by the edge's keys and by dragging it, within its bounds; hidden and shown again at the width it had; both
// kept across a reload; a page of the tree opened from it. A narrow frame starts with it hidden and shows it over the
// page, without its edge, keeping nothing. Its edge and buttons take the focus ring, and print leaves them out.
func TestUIFolderSide(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for _, rel := range []string{"docs/index.html", "docs/guide/setup.html", "docs/guide/start.html", "docs/guide/deep/x.html", "docs/api/a.html"} {
		s.write(t, rel, "<!doctype html><title>t</title><p>page</p>")
	}
	// A directory longer than the tree's height, its last page opened at the end
	for i := range 80 {
		s.write(t, fmt.Sprintf("docs/api/many/p%03d.html", i), "<p>many</p>")
	}
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "docs"))
	b.size(1200, 800, false)
	defer b.unsize()
	b.navigate("/")
	b.eval("localStorage.removeItem('frame.side')")

	// The tree's rows (a directory's state, the page open now *), the edge's value and bounds, and the room it takes
	state := `[[...document.querySelectorAll('#side-tree > *')].map((e) => e.textContent + (e.getAttribute('aria-expanded') === 'true' ? '-' : e.getAttribute('aria-expanded') ? '+' : '') + (e.getAttribute('aria-current') ? '*' : '')).join(','),
		['aria-valuenow', 'aria-valuemin', 'aria-valuemax'].map((a) => document.getElementById('side-grip').getAttribute(a)).join('/'),
		getComputedStyle(document.getElementById('frame')).marginLeft].join(' ')`
	shown := "[!document.getElementById('side').hidden, !document.getElementById('side-grip').hidden, !document.getElementById('side-show').hidden, document.getElementById('side').classList.contains('over'), !document.getElementById('frame-pick').disabled].join()"
	kept := "localStorage.getItem('frame.side')"
	key := func(k string) {
		t.Helper()

		b.eval("document.getElementById('side-grip').focus()")
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(k)); err != nil {
			t.Fatal(err)
		}
	}
	drag := func(to float64) {
		t.Helper()

		r := b.rect("document.getElementById('side-grip')")
		x, y := r.X+r.Width/2, r.Y+r.Height/2
		for _, e := range []chromedp.Action[chromedp.Void]{
			chromedp.MouseEvent(chromedp.MousePressed, x, y, chromedp.ButtonLeft, chromedp.ClickCount(1)),
			chromedp.MouseEvent(chromedp.MouseMoved, (x+to)/2, y, chromedp.ButtonLeft),
			chromedp.MouseEvent(chromedp.MouseMoved, to, y, chromedp.ButtonLeft),
			chromedp.MouseEvent(chromedp.MouseReleased, to, y, chromedp.ButtonLeft, chromedp.ClickCount(1)),
		} {
			if err := chromedp.Do(b.ctx, e); err != nil {
				t.Fatal(err)
			}
		}
	}

	out := make([]string, 0, 24)
	b.navigate("/a/docs/?file=guide%2Fsetup.html")
	out = append(out, b.until("the tree", "document.querySelectorAll('#side-tree > *').length > 3 && "+state), b.eval(shown), b.eval(kept+" ?? 'none kept'"))
	// The edge's keys: 16 px a step, the bounds at Home and End, kept as they go.
	key(kb.ArrowLeft)
	out = append(out, b.eval(state), b.eval(kept))
	key(kb.Home)
	key(kb.ArrowLeft)
	out = append(out, b.eval(state))
	key(kb.End)
	key(kb.ArrowRight)
	out = append(out, b.eval(state))
	// Dragged, to where the pointer is let go, kept once it is.
	drag(300)
	out = append(out, b.eval(state), b.eval(kept))
	drag(1000)
	out = append(out, b.eval(state))
	drag(300)
	// Hidden: the page takes the whole frame; kept across a reload; shown again at the width it had.
	b.eval("document.getElementById('side-hide').click()")
	out = append(out, b.eval(shown+" + ' ' + getComputedStyle(document.getElementById('frame')).marginLeft + ' ' + document.activeElement.id"), b.eval(kept))
	b.navigate("/a/docs/?file=guide%2Fsetup.html")
	b.until("the frame", "!document.getElementById('frame').hidden")
	out = append(out, b.eval(shown))
	b.eval("document.getElementById('side-show').click()")
	out = append(out, b.until("the tree again", "!document.getElementById('side').hidden && "+state+" + ' ' + document.activeElement.id"))
	// A directory of the tree opened, and a page of it opened from it.
	b.eval("[...document.querySelectorAll('#side-tree button.dir')].find((d) => d.textContent === 'deep').click()")
	out = append(out, b.until("deep open", "document.querySelector('#side-tree [title=\"guide/deep/x.html\"]') && "+state))
	b.eval("document.querySelector('#side-tree [title=\"guide/start.html\"]').click()")
	out = append(out, b.until("the page picked", "location.search === '?file=guide%2Fstart.html' && document.getElementById('frame-title').textContent"))

	// The edge and the hide button take the ring, reached by keyboard; print leaves the tree out.
	b.until("the tree", "!document.getElementById('side').hidden")
	for _, id := range []string{"side-hide", "side-grip"} {
		b.eval("document.getElementById(" + js(id) + ").blur(); document.body.focus()")
		for range 40 {
			if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
				t.Fatal(err)
			}
			if b.eval("document.activeElement.id") == id {
				break
			}
		}
		out = append(out, id+": "+b.settled(ringOf))
	}
	b.media("print")
	out = append(out, b.eval("['side', 'side-grip', 'side-show'].map((id) => getComputedStyle(document.getElementById(id)).display).join()"))
	b.media("")

	// Narrow: hidden at first, shown over the page without its edge, nothing kept.
	before := b.eval(kept)
	b.size(500, 800, false)
	b.navigate("/a/docs/?file=guide%2Fsetup.html")
	b.until("the frame", "!document.getElementById('frame').hidden")
	out = append(out, b.eval(shown))
	b.eval("document.getElementById('side-show').click()")
	out = append(out, b.eval(shown+" + ' ' + getComputedStyle(document.getElementById('frame')).marginLeft"), b.eval(kept+" === "+js(before)))

	ring := "solid 2px rgb(107, 105, 100) -2px seen"
	// The page open now is scrolled into the tree's sight, at the foot of a long directory too.
	b.size(1200, 800, false)
	b.navigate("/a/docs/?file=api%2Fmany%2Fp079.html")
	b.until("the tree", "document.querySelector('#side-tree [aria-current]')")
	out = append(out, b.eval("(() => { const s = document.getElementById('side').getBoundingClientRect(); "+
		"const c = document.querySelector('#side-tree [aria-current]').getBoundingClientRect(); return c.top >= s.top && c.bottom <= s.bottom; })()"))

	// The page's directory is read however the tree is: closed in the tree, the bar still picks among its pages.
	b.navigate("/a/docs/?file=guide%2Fsetup.html")
	b.until("the tree", "document.querySelector('#side-tree [aria-current]')")
	b.eval("[...document.querySelectorAll('#side-tree button.dir')].find((d) => d.textContent === 'guide').click()")
	b.until("guide closed", "!document.querySelector('#side-tree [aria-current]')")
	b.eval("document.getElementById('side-hide').click()")
	b.eval("load()")
	b.eval("document.getElementById('frame-pick').click()")
	out = append(out, b.until("the pages to pick", "[...document.querySelectorAll('#page-pick-list [role=option]')].map((o) => o.textContent).join(',')"))
	b.eval("document.getElementById('page-pick').hidePopover(); document.getElementById('side-show').click()")
	// A row with the focus keeps it as the tree is drawn anew; the title drawn is the folder's of now.
	b.eval("[...document.querySelectorAll('#side-tree button.dir')].find((d) => d.textContent === 'api').focus()")
	b.eval("items = items.map((x) => x.slug === 'docs' ? { ...x, title: 'Docs anew' } : x); renderFrame()")
	out = append(out, b.eval("document.activeElement.textContent + ' ' + document.getElementById('side-title').textContent"))
	// A directory pressed draws the tree with the folder of now, not of when the button was made.
	b.eval("items = items.map((x) => x.slug === 'docs' ? { ...x, title: 'Docs again' } : x)")
	b.eval("document.activeElement.click()")
	out = append(out, b.until("api open", "document.querySelector('#side-tree [title=\"api/many\"]') && document.getElementById('side-title').textContent"))
	// The edge steps from the width shown, which a narrower window holds under the one kept.
	b.eval("document.getElementById('side-grip').focus()")
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.End)); err != nil {
		t.Fatal(err)
	}
	b.size(700, 800, false)
	b.until("the window narrowed", "document.getElementById('side-grip').getAttribute('aria-valuenow') === '350'")
	key(kb.ArrowLeft)
	out = append(out, b.eval("document.getElementById('side-grip').getAttribute('aria-valuenow')"))
	// The folder gone, the tree goes, and the bar's pick is pressable again once the page is back.
	b.navigate("/a/docs/?file=guide%2Fsetup.html")
	b.until("the tree", "document.getElementById('frame-pick').disabled")
	docs := filepath.Join(s.Env["HOME"], "docs")
	if err := os.Rename(docs, docs+"-moved"); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.until("the folder gone", "document.getElementById('side').hidden && 'disabled ' + document.getElementById('frame-pick').disabled"))
	if err := os.Rename(docs+"-moved", docs); err != nil {
		t.Fatal(err)
	}
	b.eval("document.getElementById('side-hide').click()")
	out = append(out, b.until("the folder back", "!document.getElementById('frame-pick').hidden && 'disabled ' + document.getElementById('frame-pick').disabled"))
	// A folder's page not found has no tree, whatever the window does.
	b.navigate("/a/docs/?file=nope.html")
	b.until("the frame", "!document.getElementById('notfound').hidden")
	b.size(1100, 800, false)
	b.until("the window widened", "innerWidth === 1100")
	b.eval("new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)))")
	out = append(out, b.eval(shown))

	check(t, "side", out, "api+,guide-,deep+,setup.html*,start.html,index.html 280/160/480 280px|true,true,false,false,false|none kept|"+
		"api+,guide-,deep+,setup.html*,start.html,index.html 264/160/480 264px|{\"width\":264,\"open\":true}|"+
		"api+,guide-,deep+,setup.html*,start.html,index.html 160/160/480 160px|"+
		"api+,guide-,deep+,setup.html*,start.html,index.html 480/160/480 480px|"+
		"api+,guide-,deep+,setup.html*,start.html,index.html 300/160/480 300px|{\"width\":300,\"open\":true}|"+
		"api+,guide-,deep+,setup.html*,start.html,index.html 480/160/480 480px|"+
		"false,false,true,false,true 0px side-show|{\"width\":300,\"open\":false}|false,false,true,false,true|"+
		"api+,guide-,deep+,setup.html*,start.html,index.html 300/160/480 300px side-hide|"+
		"api+,guide-,deep-,x.html,setup.html*,start.html,index.html 300/160/480 300px|docs / guide/start.html|"+
		"side-hide: "+ring+"|side-grip: "+ring+"|none,none,none|"+
		"false,false,true,true,true|true,false,false,true,true 0px|true|true|"+
		"guide/setup.html,guide/start.html|api Docs anew|Docs again|334|disabled false|disabled false|false,false,false,false,true")
}
