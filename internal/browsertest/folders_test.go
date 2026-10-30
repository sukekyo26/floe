package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/link"
)

// TestUIFolders opens a linked folder's tree in the list: its URL leads to the list with it open, a directory is read
// only once opened, what is open is kept in the browser across a reload and not in the URL (a name with , and : too),
// what is no longer there leaves what is kept, and it closes with its parent; opened and closed, only the rows under it
// are added or taken away, the others the same elements and not rising in again; a page added shows at the next poll,
// and a folder gone says so. A page of it opens in the frame under the folder's
// title, its CSS served beside it, a move to a part of it written into the frame's URL, its pages to pick from those of
// its directory. A folder added from the dialog is opened.
func TestUIFolders(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	page := func(rel, css string) {
		s.write(t, rel, `<!doctype html><title>t</title><link rel="stylesheet" href="`+css+`s.css">`+
			`<h1 id="top">x</h1><p style="height: 3000px"></p><h2 id="end">e</h2>`+
			// It reports the part it is at, as the frame cannot read the sandboxed page, and goes to the part it is sent.
			`<script>addEventListener('message', (e) => { if (e.data.go) location.hash = e.data.go; }); `+
			`setInterval(() => top.postMessage({ at: location.hash }, '*'), 50)</script>`)
	}
	page("docs/index.html", "")
	page("docs/guide/setup.html", "../")
	page("docs/guide/start.html", "../")
	page("docs/guide/deep/x.html", "../../")
	page("docs/a,b:c/odd.html", "../")
	s.write(t, "docs/s.css", "h1 { color: rgb(1, 2, 3); }")
	// A directory of no page, whose note the keys pass over; one of more pages than a directory lists, whose last opens all the same.
	s.write(t, "docs/assets/a.css", "")
	for i := range link.MaxListed + 1 {
		s.write(t, fmt.Sprintf("docs/big/p%04d.html", i), "<p>big</p>")
	}
	docs := filepath.Join(s.Env["HOME"], "docs")
	s.floe(t, "publish", "--link", docs)

	// The rows under the folder, a directory's name with its state, and the directories read so far
	rows := `[...document.querySelectorAll('#list li.tree')].map((li) => {
		const b = li.querySelector('button.title');
		return b ? b.textContent + (b.getAttribute('aria-expanded') === 'true' ? '-' : '+') : li.querySelector('.title')?.textContent ?? '(' + li.textContent.trim() + ')';
	}).join(',')`
	read := `performance.getEntriesByType('resource').map((e) => new URL(e.name)).filter((u) => u.pathname.endsWith('/pages'))
		.map((u) => u.searchParams.get('dir') || '.').filter((d, i, all) => all.indexOf(d) === i).sort().join(',')`
	opens := "localStorage.getItem('list.open') + ' ' + location.search"
	click := func(name string) {
		t.Helper()

		b.eval(`[...document.querySelectorAll('#list li.tree button.title')].find((x) => x.textContent === ` + js(name) + `).click()`)
	}

	out := make([]string, 0, 24)
	b.navigate("/")
	b.eval("localStorage.removeItem('list.open')")
	b.navigate("/a/docs/")
	out = append(out, b.until("the folder open", "location.pathname === '/' && document.querySelectorAll('#list li.tree').length > 0 && "+opens))
	// The folders' tab has no project groups: each folder heads its tree.
	out = append(out, b.eval(rows), b.eval("document.querySelectorAll('#list li.group').length + ' groups, ' + document.querySelectorAll('#list li.folder').length + ' folder'"))
	click("guide")
	b.until("guide open", "document.querySelectorAll('#list li.tree').length > 3")
	click("a,b:c")
	b.until("a,b:c open", rows+".includes('odd.html')")
	out = append(out, b.eval(opens))
	// Reloaded, what was open is again, read once each; the directory under guide, never opened, is not read. A folder
	// not registered and a directory not there leave what is kept.
	// Written where no list is drawn, which would keep what it has over it
	back := b.eval("location.pathname + location.search")
	b.navigate("/api/artifacts")
	b.eval("localStorage.setItem('list.open', JSON.stringify([...JSON.parse(localStorage.getItem('list.open')), 'gone', 'docs:nope']))")
	b.navigate(back)
	out = append(out, b.until("the tree again", rows+".includes('odd.html') && "+rows),
		b.until("the keys gone", "!localStorage.getItem('list.open').includes('nope') && "+opens), b.eval(read))
	// Closed and opened again, the rows under guide go and come, the others the same elements, the focus where it was;
	// only the rows added rise in, and a redraw raises none.
	b.still()
	b.eval(`window.marked = [...document.querySelectorAll('#list > li')]; window.rose = [];
		document.getElementById('list').addEventListener('animationstart', (e) => rose.push(e.target.dataset.key));
		[...document.querySelectorAll('#list li.tree button.title')].find((x) => x.textContent === 'guide').focus()`)
	click("guide")
	b.until("guide closed", "!"+rows+".includes('setup')")
	out = append(out, b.eval("marked.filter((li) => !li.isConnected).map((li) => li.dataset.key).join(',') + ' gone, focus ' + document.activeElement.textContent"))
	click("guide")
	b.until("guide open", "rose.length > 0")
	b.still()
	out = append(out, b.eval("[...document.querySelectorAll('#list > li')].filter((li) => !marked.includes(li)).map((li) => li.dataset.key).join(',') + ' new, rose ' + rose.join(',')"))
	b.eval("rose = []; render()")
	b.still()
	out = append(out, b.eval("'redrawn, rose ' + rose.length"))
	// Closed, guide's rows go, and it leaves the URL.
	click("guide")
	out = append(out, b.until("guide closed", "!"+rows+".includes('setup') && "+rows), b.eval(opens))
	// ↓ passes over a note, which has nothing to focus, to the row after it.
	click("assets")
	b.until("assets open", rows+".includes('(No HTML')")
	b.eval(`[...document.querySelectorAll('#list li.tree button.title')].find((x) => x.textContent === 'assets').focus()`)
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.ArrowDown)); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("document.activeElement.textContent"))
	click("assets")
	// A page added shows at the next poll.
	page("docs/new.html", "")
	out = append(out, b.until("the page added", rows+".includes('new.html') && "+rows))

	// A page of the folder in the frame: under the folder's title, its own URL loaded, its CSS beside it.
	b.navigate("/a/docs/?file=guide%2Fsetup.html#end")
	out = append(out, b.until("the frame", "!document.getElementById('frame').hidden && document.getElementById('frame-title').textContent"),
		b.eval("new URL(document.getElementById('frame').src).pathname + new URL(document.getElementById('frame').src).hash"),
		b.eval("['frame-star', 'frame-archive', 'frame-download'].map((id) => document.getElementById(id).hidden).join()"),
		b.eval("[...document.querySelectorAll('#page-pick-list [role=option]')].length === 0 && 'none yet'"))
	// A move to another part of it, as the page tells it, is written into the frame's URL.
	// The URL is the frame's own to make: a relative one would be read against the frame URL.
	// The part it is at goes into the frame's URL, and the frame's own part, from the address bar, to the page.
	b.eval("addEventListener('message', (e) => { if (e.data.at !== undefined) window.at = e.data.at; })")
	// Sent again at each look, as the page may not listen yet.
	out = append(out, b.until("the part in the URL", "(document.getElementById('frame').contentWindow.postMessage({ go: '#top' }, '*'), "+
		"location.hash === '#top' && window.at === '#top') && location.search + location.hash"))
	b.eval("location.hash = '#end'")
	out = append(out, b.until("the part in the page", "window.at === '#end' && 'the page at #end'"))
	// The bar picks among the pages with the folder's tree hidden: beside it, the tree lists them.
	b.eval("document.getElementById('side-hide').click(); document.getElementById('frame-pick').click()")
	out = append(out, b.until("the pages to pick", "[...document.querySelectorAll('#page-pick-list [role=option]')].map((o) => o.textContent + (o.getAttribute('aria-current') ? '*' : '')).join(',')"))
	b.navigate("/a/docs/guide/setup.html")
	out = append(out, b.eval("getComputedStyle(document.querySelector('h1')).color"))

	// A page past what its directory lists opens, as the server serves it.
	b.navigate("/a/docs/?file=big%2Fp1000.html")
	out = append(out, b.until("the page past the list", "!document.getElementById('frame').hidden && document.getElementById('frame-title').textContent"))

	// Gone, the folder says so and opens nothing.
	if err := os.Rename(docs, docs+"-moved"); err != nil {
		t.Fatal(err)
	}
	b.navigate("/?tab=folders&open=docs")
	out = append(out, b.until("the folder missing", "document.querySelector('#list .path.missing') !== null && 'tree rows ' + document.querySelectorAll('#list li.tree').length"))
	if err := os.Rename(docs+"-moved", docs); err != nil {
		t.Fatal(err)
	}

	// Added from the dialog, a folder is opened.
	s.write(t, "more/m.html", "<title>m</title>")
	b.navigate("/?tab=links")
	// Opened for a folder, the dialog shows the folder's icon in place of the link's.
	b.eval("document.getElementById('add-folder').click()")
	out = append(out, b.eval("['add-icon-link', 'add-icon-folder'].map((id) => getComputedStyle(document.getElementById(id)).display).join()"))
	b.eval(`document.getElementById('add-path').value = ` + js(filepath.Join(s.Env["HOME"], "more")) + `;
		document.getElementById('add').requestSubmit()`)
	out = append(out, b.until("the folder added", rows+".includes('m.html') && new URLSearchParams(location.search).get('tab') + ' ' + "+opens),
		b.eval("document.getElementById('add-title').textContent"))
	// Once the rows have risen in, every folder's heading and every row of the tree shows whole.
	b.still()
	out = append(out, b.eval("[...document.querySelectorAll('#list > li')].map((li) => li.className.split(' ')[0] + ':' + getComputedStyle(li).opacity).join(',')"))

	guide := "dir:docs:guide/deep,page:docs:guide/setup.html,page:docs:guide/start.html"
	check(t, "folders", out, `["docs"] ?tab=folders|a,b:c+,assets+,big+,guide+,index.html|0 groups, 1 folder|["docs","docs:guide","docs:a,b:c"] ?tab=folders|`+
		"a,b:c-,odd.html,assets+,big+,guide-,deep+,setup.html,start.html,index.html|"+`["docs","docs:guide","docs:a,b:c"] ?tab=folders|.,a,b:c,guide|`+
		guide+" gone, focus guide|"+guide+" new, rose "+guide+"|redrawn, rose 0|"+
		`a,b:c-,odd.html,assets+,big+,guide+,index.html|["docs","docs:a,b:c"] ?tab=folders|big|`+"a,b:c-,odd.html,assets+,big+,guide+,index.html,new.html|"+
		"docs / guide/setup.html|/a/docs/guide/setup.html#end|true,true,true|none yet|"+
		"?file=guide%2Fsetup.html#top|the page at #end|guide/setup.html*,guide/start.html|rgb(1, 2, 3)|docs / big/p1000.html|tree rows 0|"+
		"none,block|"+`folders ["docs","docs:a,b:c","more"] ?tab=folders|Add a folder|`+"folder:1,tree:1,folder:1"+strings.Repeat(",tree:1", 7))
}
