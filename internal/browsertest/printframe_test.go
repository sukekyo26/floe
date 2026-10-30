package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
)

// printed returns, read with the page drawn for print or for the screen, which of the frame's parts show (the bar,
// the frame, the pages to pick from, the error, the missing page's state, the note naming the page alone), and the
// note's words and URL: "name:shown" each, a word for each, then the words and the URL.
const printed = `(() => {
  const shown = (s) => { const e = document.querySelector(s); return e !== null && e.getClientRects().length > 0; };
  return [['bar', '.bar'], ['frame', '#frame'], ['pick', '#page-pick'], ['error', '#error'], ['notfound', '#notfound'], ['note', '#print-alone']]
    .map(([n, s]) => n + ':' + shown(s)).join(' ') + ' | ' + (document.getElementById('print-alone-text')?.textContent ?? '') +
    ' | ' + (document.getElementById('print-raw')?.textContent ?? '');
})()`

// TestUIPrintFrame prints the frame of a page, a frame before the list is read, one whose list failed, a missing
// page's and the list, in English and in Japanese: a frame names the page alone, at its URL, in the reader's
// language, in place of the bar, the frame and the pages to pick from, as an iframe prints one screen of the page in
// it; a missing page's frame prints its state alone, and the list as it shows. The screen shows no note.
func TestUIPrintFrame(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")
	for _, lang := range []i18n.Lang{i18n.English, i18n.Lang("ja")} {
		b := newBrowser(t, s.Base, lang)
		c := i18n.New(lang)
		words := c.Msg("ui_print_alone", c.Msg("ui_open_raw"))
		raw := s.Base + "/a/p/?raw"

		b.navigate("/a/p/")
		b.until("the frame", "!document.getElementById('frame').hidden")
		// The pages to pick from, open, as Ctrl+P may find them
		b.eval("document.getElementById('frame-pick').click()")
		b.until("the pages to pick from", "document.getElementById('page-pick').matches(':popover-open')")
		b.media("print")
		got := make([]string, 0, 6)
		got = append(got, b.eval(printed))
		b.media("")
		got = append(got, b.eval(printed))
		b.navigate("/a/none/")
		b.until("the missing page", "!document.getElementById('notfound').hidden")
		b.media("print")
		got = append(got, b.eval(printed))
		b.navigate("/")
		got = append(got, b.eval(printed))
		b.media("")

		// A list never read, then one whose read fails.
		pending := b.addScript("if (" + managementPage + ") window.fetch = () => new Promise(() => {})")
		b.navigate("/a/p/")
		b.media("print")
		got = append(got, b.eval(printed))
		b.media("")
		b.removeScript(pending)
		failing := b.addScript("if (" + managementPage + ") window.fetch = () => Promise.reject(new TypeError('Failed to fetch'))")
		b.navigate("/a/p/")
		b.until("the error", "!document.getElementById('error').hidden")
		b.media("print")
		got = append(got, b.eval(printed))
		b.media("")
		b.removeScript(failing)

		note := " | " + words + " | " + raw
		check(t, "print "+string(lang), got, "bar:false frame:false pick:false error:false notfound:false note:true"+note+"|"+
			"bar:true frame:true pick:true error:false notfound:false note:false"+note+"|"+
			"bar:false frame:false pick:false error:false notfound:true note:false | "+words+" | "+s.Base+"/a/none/?raw|"+
			"bar:true frame:false pick:false error:false notfound:false note:false | "+words+" | |"+
			"bar:false frame:false pick:false error:false notfound:false note:true"+note+"|"+
			"bar:false frame:false pick:false error:true notfound:false note:true"+note)
	}
}
