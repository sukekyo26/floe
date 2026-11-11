package browsertest_test

import (
	"fmt"
	"strings"
	"testing"
)

// sidePage is a long page of three sections of paragraphs, those of changed whose words differ, and a script that
// tells the top page where it went: its fragment, and whether what it names is in view.
func sidePage(changed map[int]string) string {
	var b strings.Builder
	for i := range 45 {
		if i%15 == 0 {
			fmt.Fprintf(&b, "<h2>Part %d</h2>", i/15+1)
		}
		words := "Paragraph %d of the page, the same in both."
		if w, ok := changed[i]; ok {
			words = w
		}
		fmt.Fprintf(&b, "<p style=\"height: 120px\">"+words+"</p>", i)
	}
	b.WriteString("<script>setInterval(() => { const el = document.getElementById(decodeURIComponent(location.hash.slice(1))); " +
		"const r = el?.getBoundingClientRect(); parent.postMessage('side:' + location.hash + ':' + Boolean(r && r.top >= 0 && r.top < innerHeight), '*'); }, 30)</script>")
	return b.String()
}

// sideState is where each of the two pages went last, as "left right".
const sideState = `(() => { const f = (id) => (window.sideAt ?? {})[id] ?? '-'; return f('diff-frame-before') + ' ' + f('diff-frame'); })()`

// TestUIDiffSide shows two versions side by side: n, p and the contents move both to the same place, each in view
// on its own page, and the way to show it is kept.
func TestUIDiffSide(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "side/s.yaml", "S", sidePage(map[int]string{
		5: "Paragraph %d said one thing.", 20: "Paragraph %d of an old version.", 35: "Paragraph %d to be changed.",
	})), "--slug", "s")
	s.floe(t, "publish", s.page(t, "side/s.yaml", "S", sidePage(map[int]string{
		5: "Paragraph %d said another thing.", 20: "Paragraph %d of a new version.", 35: "Paragraph %d changed.",
	})), "--slug", "s")
	b.size(1440, 700, false)
	b.navigate("/a/s/?diff")
	b.until("what changed", "typeof diffAnswer !== 'undefined' && diffAnswer !== null")
	b.eval(`window.sideAt = {}; addEventListener('message', (e) => {
		if (typeof e.data !== 'string' || !e.data.startsWith('side:')) return;
		const f = [...document.querySelectorAll('.dbody > iframe')].find((x) => x.contentWindow === e.source);
		if (f) window.sideAt[f.id] = e.data.slice(5);
	})`)
	out := make([]string, 0, 6)
	b.eval("document.querySelector('[data-diff-view=side]').click()")
	out = append(out, b.until("both pages", "!document.getElementById('diff-frame-before').hidden && !"+sideState+".includes('-') && diffAnswer.counts.places + ' ' + "+sideState))
	at := func(what, want string) string {
		return b.until(what, sideState+" === '"+want+"' && "+sideState)
	}
	b.pressKey("n")
	out = append(out, at("both at the first place", "#floe-diff.0:true #floe-diff.0:true"))
	b.pressKey("n")
	b.pressKey("n")
	out = append(out, at("both at the third place", "#floe-diff.2:true #floe-diff.2:true"))
	b.pressKey("p")
	out = append(out, at("both back at the second", "#floe-diff.1:true #floe-diff.1:true"))
	// The contents: the first part, by its first place.
	b.eval("document.querySelector('#diff-toc button[data-section]').click()")
	out = append(out, at("both at the first part", "#floe-diff.0:true #floe-diff.0:true"))
	// Kept: opened again, side by side.
	b.navigate("/a/s/?diff")
	out = append(out, b.until("side by side again", "typeof diffAnswer !== 'undefined' && diffAnswer !== null && !document.getElementById('diff-frame-before').hidden && "+
		"document.querySelector('[data-diff-view=side]').getAttribute('aria-pressed')"))
	check(t, "side", out, "3 :false :false|#floe-diff.0:true #floe-diff.0:true|#floe-diff.2:true #floe-diff.2:true|#floe-diff.1:true #floe-diff.1:true|#floe-diff.0:true #floe-diff.0:true|true")
}
