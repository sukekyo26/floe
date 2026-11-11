package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// syncPage is a long page of n paragraphs, those with an id in both versions, and a script that tells the top page
// where it is: its scroll and the id of the paragraph at its top.
func syncPage(added string) string {
	var b strings.Builder
	for i := range 60 {
		fmt.Fprintf(&b, "<p id=\"p%d\" style=\"height: 120px\">Paragraph %d of the page.</p>", i, i)
		if i == 10 {
			b.WriteString(added)
		}
	}
	b.WriteString("<script>setInterval(() => { const at = document.elementFromPoint(innerWidth / 2, 2)?.closest('[id^=p]'); " +
		"parent.postMessage('sync:' + scrollY + ':' + (at ? at.id : ''), '*'); }, 30)</script>")
	return b.String()
}

// syncState is where each of the two pages last told it is, as "left right", by the id at their top.
const syncState = `(() => { const f = (id) => (window.syncAt ?? {})[id] ?? '-'; return f('diff-frame-before') + ' ' + f('diff-frame'); })()`

// TestUIDiffScrollTogether shows two versions side by side, the later longer: the wheel over either page moves the
// other to the same paragraph, the one moved tells nothing back so that both stay still, and a message from another
// window, or of an origin not null, moves nothing.
func TestUIDiffScrollTogether(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "sync/s.yaml", "S", syncPage("")), "--slug", "s")
	added := strings.Repeat(`<p style="height: 120px">Only in the later version.</p>`, 8)
	s.floe(t, "publish", s.page(t, "sync/s.yaml", "S", syncPage(added)), "--slug", "s")
	b.size(1440, 700, false)
	b.navigate("/a/s/?diff")
	b.until("what changed", "typeof diffAnswer !== 'undefined' && diffAnswer !== null")
	b.eval(`window.syncAt = {}; addEventListener('message', (e) => {
		if (typeof e.data !== 'string' || !e.data.startsWith('sync:')) return;
		const f = [...document.querySelectorAll('.dbody > iframe')].find((x) => x.contentWindow === e.source);
		if (f) window.syncAt[f.id] = e.data.split(':')[2];
	});
	window.told = {}; addEventListener('message', (e) => {
		const f = [...document.querySelectorAll('.dbody > iframe')].find((x) => x.contentWindow === e.source);
		if (f && e.data?.artifactDiffAt) window.told[f.id] = (window.told[f.id] ?? 0) + 1;
	})`)
	b.eval("document.querySelector('[data-diff-view=side]').click()")
	b.until("both pages", "!document.getElementById('diff-frame-before').hidden && !"+syncState+".includes('-')")
	out := make([]string, 0, 6)
	wheel := func(id string, dy float64) {
		r := b.rect("document.getElementById('" + id + "')")
		e := input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseWheel, X: r.X + r.Width/2, Y: r.Y + r.Height/2, DeltaX: 1e-6, DeltaY: dy}
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, e); err != nil {
			t.Fatal(err)
		}
	}
	// Both at one paragraph, and staying there: the same state a while after.
	last := ""
	together := func(what string) string {
		got := b.until(what, "(() => { const [l, r] = "+syncState+".split(' '); return l !== '' && l === r && "+syncState+" !== '"+last+"' && "+syncState+"; })()")
		last = got
		b.still()
		if again := b.eval(syncState); again != got {
			t.Errorf("%s: moved on from %s to %s", what, got, again)
		}
		return got
	}
	// The later page past what it alone holds, the earlier following it; then the earlier, the later following.
	// Each time, the page moved tells nothing: only the one the wheel moved.
	told := "[(window.told['diff-frame-before'] ?? 0) > 0, (window.told['diff-frame'] ?? 0) > 0].join(' ')"
	b.eval("window.told = {}")
	wheel("diff-frame", 3600)
	out = append(out, together("the earlier to follow"), b.eval(told))
	b.eval("window.told = {}")
	wheel("diff-frame-before", 1200)
	out = append(out, together("the later to follow"), b.eval(told))
	// Told from the top page itself, of its own origin, and from a sandboxed frame not the diff's: nothing moves.
	before := b.eval(syncState)
	b.eval(`postMessage({ artifactDiffAt: { pair: 1, off: 0 } }, '*');
		const f = document.createElement('iframe');
		f.sandbox = 'allow-scripts';
		f.srcdoc = "<script>parent.postMessage({ artifactDiffAt: { pair: 1, off: 0 } }, '*')<\/script>";
		document.body.append(f)`)
	b.still()
	out = append(out, fmt.Sprint(b.eval(syncState) == before))
	check(t, "scroll together", out, "p18 p18|false true|p27 p27|true false|true")
}
