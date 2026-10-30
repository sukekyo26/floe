package browsertest_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUIFolders opens a linked folder's tree in the list: its URL leads to the list with it open, a directory is read
// only once opened, what is open stays in the URL across a reload (a name with , and : too) and closes with its parent,
// a page added shows at the next poll, and a folder gone says so. A page of it opens in the frame under the folder's
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
	docs := filepath.Join(s.Env["HOME"], "docs")
	s.floe(t, "publish", "--link", docs)

	// The rows under the folder, a directory's name with its state, and the directories read so far
	rows := `[...document.querySelectorAll('#list li.tree')].map((li) => {
		const b = li.querySelector('button.title');
		return b ? b.textContent + (b.getAttribute('aria-expanded') === 'true' ? '-' : '+') : li.querySelector('.title')?.textContent ?? li.textContent.trim();
	}).join(',')`
	read := `performance.getEntriesByType('resource').map((e) => new URL(e.name)).filter((u) => u.pathname.endsWith('/pages'))
		.map((u) => u.searchParams.get('dir') || '.').filter((d, i, all) => all.indexOf(d) === i).sort().join(',')`
	opens := "new URLSearchParams(location.search).getAll('open').join(',')"
	click := func(name string) {
		t.Helper()

		b.eval(`[...document.querySelectorAll('#list li.tree button.title')].find((x) => x.textContent === ` + js(name) + `).click()`)
	}

	out := make([]string, 0, 16)
	b.navigate("/a/docs/")
	out = append(out, b.until("the folder open", "location.pathname === '/' && document.querySelectorAll('#list li.tree').length > 0 && location.search"))
	out = append(out, b.eval(rows))
	click("guide/")
	b.until("guide open", "document.querySelectorAll('#list li.tree').length > 3")
	click("a,b:c/")
	b.until("a,b:c open", rows+".includes('odd.html')")
	out = append(out, b.eval(opens))
	// Reloaded, what was open is again, read once each; the directory under guide, never opened, is not read.
	b.navigate(b.eval("location.pathname + location.search"))
	out = append(out, b.until("the tree again", rows+".includes('odd.html') && "+rows), b.eval(read))
	// Closed, guide's rows go, and it leaves the URL.
	click("guide/")
	out = append(out, b.until("guide closed", "!"+rows+".includes('setup') && "+rows), b.eval(opens))
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
	b.eval("document.getElementById('frame-pick').click()")
	out = append(out, b.until("the pages to pick", "[...document.querySelectorAll('#page-pick-list [role=option]')].map((o) => o.textContent + (o.getAttribute('aria-current') ? '*' : '')).join(',')"))
	b.navigate("/a/docs/guide/setup.html")
	out = append(out, b.eval("getComputedStyle(document.querySelector('h1')).color"))

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
	b.eval(`document.getElementById('add-folder').click();
		document.getElementById('add-path').value = ` + js(filepath.Join(s.Env["HOME"], "more")) + `;
		document.getElementById('add').requestSubmit()`)
	out = append(out, b.until("the folder added", rows+".includes('m.html') && new URLSearchParams(location.search).get('tab') + ' ' + "+opens),
		b.eval("document.getElementById('add-title').textContent"))

	check(t, "folders", out, "?tab=folders&open=docs|a,b:c/+,guide/+,index.html|docs,docs:guide,docs:a,b:c|"+
		"a,b:c/-,odd.html,guide/-,deep/+,setup.html,start.html,index.html|.,a,b:c,guide|"+
		"a,b:c/-,odd.html,guide/+,index.html|docs,docs:a,b:c|a,b:c/-,odd.html,guide/+,index.html,new.html|"+
		"docs / guide/setup.html|/a/docs/guide/setup.html#end|true,true,true|none yet|"+
		"?file=guide%2Fsetup.html#top|the page at #end|guide/setup.html*,guide/start.html|rgb(1, 2, 3)|tree rows 0|"+
		"folders more|Add a folder")
}
