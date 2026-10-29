package browsertest_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// copyPage is a page of lang with a part of each kind that copies: code ending in a line break, a terminal whose
// last command ends in one, and a diff that does not, whose head's faded line runs under the copy button. Its script reports the copy button of the code, as the
// center of its box, its status, and whether it has the focus and shows it, to a frame around it.
const copyPage = `template: report
lang: %[1]s
title: Copy
sections:
  - heading: Copy
    parts:
      - code: { file: main.go, lang: go, text: "package main\n\nfunc main() {}\n" }
      - terminal:
          shell: zsh
          steps:
            - { run: go test ./..., out: ok }
            - { run: "git commit \\\n  -m \"fix\"" }
            - { run: "echo one\n" }
      - diff: { file: a.go, text: "diff --git a/%[2]s b/%[2]s\n@@ -1,2 +1,2 @@\n package a\n-var x = 1\n+var x = 2" }
      - html: |
          <script>
          document.addEventListener('pointerover', () => parent.postMessage('over', '*'));
          setInterval(() => {
            const button = document.querySelector('[data-floe-part=code] .floe-copy');
            const r = button.getBoundingClientRect();
            parent.postMessage(JSON.stringify({
              x: r.x + r.width / 2, y: r.y + r.height / 2,
              status: document.querySelector('[data-floe-part=code] .floe-copy-status').textContent,
              focused: document.activeElement === button, shown: button.matches(':focus-visible'),
            }), '*');
          }, 100);
          </script>
`

// longPath is the path of the diff of copyPage, long enough that its line runs under the button.
var longPath = strings.Repeat("internal/kit/floe/parts/", 8) + "part.html"

// copied is what the button of each part copies.
var copied = map[string]string{
	"code":     "package main\n\nfunc main() {}\n",
	"terminal": "go test ./...\ngit commit \\\n  -m \"fix\"\necho one\n",
	"diff":     "diff --git a/" + longPath + " b/" + longPath + "\n@@ -1,2 +1,2 @@\n package a\n-var x = 1\n+var x = 2\n",
}

// copySite is a site with the page of copyPage in lang published as copy, whose tab may read the clipboard on
// the server's origin and is always focused, as the clipboard asks.
func copySite(t *testing.T, lang string) *site {
	t.Helper()

	s := newSite(t)
	s.floe(t, "publish", s.write(t, "copy.yaml", fmt.Sprintf(copyPage, lang, longPath)), "--slug", "copy")
	id := chromedp.FromContext(s.b.ctx).BrowserContextID
	for _, name := range []string{"clipboard-read", "clipboard-write"} {
		grant := cdpbrowser.SetPermissionParams{
			Permission: &cdpbrowser.PermissionDescriptor{Name: name}, Setting: cdpbrowser.PermissionSettingGranted,
			Origin: s.Base, BrowserContextID: id,
		}
		if _, err := chromedp.CallBrowser(s.b.ctx, cdpbrowser.SetPermission, grant); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := chromedp.Call(s.b.ctx, emulation.SetFocusEmulationEnabled, emulation.SetFocusEmulationEnabledParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return s
}

// clip empties the clipboard, runs press, and returns what the clipboard then holds, read on the management page.
func (s *site) clip(t *testing.T, press func()) string {
	t.Helper()

	s.b.navigate("/")
	s.b.eval("navigator.clipboard.writeText('')")
	press()
	s.b.navigate("/")
	return s.b.eval("navigator.clipboard.readText()")
}

// button is the copy button of part on the page shown.
func button(part string) string {
	return "document.querySelector('[data-floe-part=" + part + "] .floe-copy')"
}

// status is the status of the copy button of part on the page shown.
func status(part string) string {
	return "document.querySelector('[data-floe-part=" + part + "] .floe-copy-status').textContent"
}

// pressButton clicks the copy button of part on the page shown, as a mouse does.
func (b *browser) pressButton(part string) {
	b.t.Helper()

	b.until("the copy button of "+part, button(part))
	b.eval(button(part) + ".scrollIntoView({ block: 'center' })")
	r := b.rect(button(part))
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
}

// gesture runs expr as if the reader had just clicked: the page may write the clipboard.
func (b *browser) gesture(expr string) {
	b.t.Helper()

	yes := func(p *runtime.EvaluateParams) {
		gesture := true
		p.UserGesture = &gesture
	}
	if _, err := chromedp.Run(b.ctx, chromedp.Evaluate[bool]("("+expr+", true)", yes)); err != nil {
		b.t.Fatalf("%s: %v", expr, err)
	}
}

// report is what the page of copyPage in a frame tells of its code's copy button.
type report struct {
	X, Y           float64
	Status         string
	Focused, Shown bool
}

// frameReport waits for the page in a frame to report its code's copy button, until is true of it, and returns it.
func (b *browser) frameReport(what, until string) report {
	b.t.Helper()

	b.eval("window.copyReports ||= (addEventListener('message', (e) => { if (String(e.data).startsWith('{')) window.copyReport = JSON.parse(e.data); }), true)")
	got := b.until(what, "window.copyReport && ("+until+")(window.copyReport) && JSON.stringify(window.copyReport)")
	var r report
	if err := json.Unmarshal([]byte(got), &r); err != nil {
		b.t.Fatal(err)
	}
	return r
}

// pressInFrame clicks the copy button of the code of the page in the frame, the element of the page shown that
// frame names.
func (b *browser) pressInFrame(frame string) report {
	b.t.Helper()

	box := b.rect(frame)
	r := b.frameReport("the page in the frame", "(r) => true")
	b.hover(box.X+r.X, box.Y+r.Y)
	b.click(box.X+r.X, box.Y+r.Y)
	return b.frameReport("the copy in the frame", "(r) => r.status !== ''")
}

// TestUICopyRaw presses the copy button of code, a terminal and a diff, with the mouse and with the keyboard: each
// copies its text, says so, and comes back after 1.5 seconds; the focus ring is drawn in the colour of the code,
// inside the button, which lies on the code's ground inside its part.
func TestUICopyRaw(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	for _, part := range []string{"code", "terminal", "diff"} {
		got := s.clip(t, func() {
			b.navigate("/a/copy/?raw")
			b.pressButton(part)
			b.until("the copy of "+part, status(part)+" === 'Copied'")
		})
		if got != copied[part] {
			t.Errorf("%s by the mouse copies %q, want %q", part, got, copied[part])
		}
		var look string
		got = s.clip(t, func() {
			b.navigate("/a/copy/?raw")
			for range 50 {
				if b.eval("document.activeElement === "+button(part)) == "true" {
					break
				}
				b.pressKey(kb.Tab)
			}
			look = b.eval(`(() => {
				const button = ` + button(part) + `, part = button.closest('[data-floe-part]'), pre = part.querySelector('pre');
				const style = getComputedStyle(button), ground = getComputedStyle(pre).backgroundColor;
				const inside = (a, b) => a.left >= b.left && a.right <= b.right && a.top >= b.top && a.bottom <= b.bottom;
				return [
					document.activeElement === button && button.matches(':focus-visible'),
					style.outlineStyle, style.outlineColor === getComputedStyle(pre).color, parseFloat(style.outlineOffset) < 0,
					style.backgroundColor === (ground === 'rgba(0, 0, 0, 0)' ? getComputedStyle(part).backgroundColor : ground),
					inside(button.getBoundingClientRect(), pre.getBoundingClientRect()), button.getAttribute('aria-label'),
					button.contains(document.elementFromPoint(...(({ x, y, width, height }) => [x + width / 2, y + height / 2])(button.getBoundingClientRect()))),
				].join();
			})()`)
			b.pressKey(kb.Enter)
			b.until("the copy of "+part+" by the keyboard", status(part)+" === 'Copied'")
		})
		if got != copied[part] {
			t.Errorf("%s by the keyboard copies %q, want %q", part, got, copied[part])
		}
		if want := "true,solid,true,true,true,true,Copy,true"; look != want {
			t.Errorf("%s: the focused button is %s, want %s (focus shown, ring, its colour the code's, inside, ground the code's, in the code, label, on top)", part, look, want)
		}
	}

	// After 1.5 seconds the button is as before.
	b.navigate("/a/copy/?raw")
	b.pressButton("code")
	b.until("the copy", status("code")+" === 'Copied' && "+button("code")+".classList.contains('copied')")
	b.until("the button to come back", status("code")+" === '' && "+button("code")+".getAttribute('aria-label') === 'Copy' && "+
		button("code")+".title === 'Copy' && !"+button("code")+".classList.contains('copied')")

	// A second press starts the 1.5 seconds again: it comes back no sooner than 1.5 s after the second press, on the
	// page's clock (a timer never fires early).
	b.navigate("/a/copy/?raw")
	b.eval(watch)
	b.pressButton("code")
	b.until("the copy", status("code")+" === 'Copied'")
	b.pressButton("code")
	back := b.until("the button to come back after the second press", `(() => {
		const p = log.filter((e) => e[0] === 'press').at(-1)[1], after = log.filter((e) => e[1] > p);
		const back = after.slice(after.findIndex((e) => e[0] === 'Copied') + 1).find((e) => e[0] === '');
		return back && after.some((e) => e[0] === 'Copied') ? String(back[1] - p) : '';
	})()`)
	if ms, err := strconv.ParseFloat(back, 64); err != nil || ms < 1499 {
		t.Errorf("the button came back %s ms after a second press, want 1500 or more", back)
	}

	// A press whose copy ends after a later press's shows nothing.
	b.navigate("/a/copy/?raw")
	b.eval(watch)
	b.eval("{ let n = 0; navigator.clipboard.writeText = () => new Promise((done) => setTimeout(() => { log.push(['written', performance.now()]); done(); }, n++ === 0 ? 1000 : 0)); }")
	b.pressButton("code")
	b.pressButton("code")
	last := b.until("the first copy to end", "log.filter((e) => e[0] === 'written').length === 2 && String(log.filter((e) => e[0] === 'written')[1][1])")
	b.until("a moment after it", "performance.now() - "+last+" > 200")
	if got := b.eval("log.filter((e) => e[1] > " + last + " && e[0] !== '').map((e) => e[0]).join()"); got != "" {
		t.Errorf("after the first press's copy ended, the status changed to %q, want nothing", got)
	}
}

// watch records each press of the copy button of the code and each change of its status, with the page's clock, in
// window.log.
var watch = `(() => {
	window.log = [];
	const button = ` + button("code") + `, status = button.parentElement.querySelector('.floe-copy-status');
	button.addEventListener('click', () => log.push(['press', performance.now()]), true);
	new MutationObserver(() => log.push([status.textContent, performance.now()])).observe(status, { childList: true, characterData: true, subtree: true });
})()`

// TestUICopyFrame presses the copy button of code in the frame of the management page, which lets the page write
// the clipboard.
func TestUICopyFrame(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	got := s.clip(t, func() {
		b.navigate("/a/copy/")
		if r := b.pressInFrame("document.getElementById('frame')"); r.Status != "Copied" {
			t.Errorf("the status is %q, want Copied", r.Status)
		}
	})
	if got != copied["code"] {
		t.Errorf("copies %q, want %q", got, copied["code"])
	}
}

// TestUICopyFallback presses the copy button of code in a frame that does not let the page write the clipboard,
// as VS Code's Simple Browser frames the management page: the copy command copies, and the focus stays on the
// button, not shown.
func TestUICopyFallback(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	// Another origin, framing the page as Simple Browser frames the management page (simpleBrowserView.ts).
	vscode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<iframe id="frame" sandbox="allow-scripts allow-forms allow-same-origin allow-downloads" src="`+ //nolint:errcheck // the test checks the copy
			s.Base+`/a/copy/?raw" style="position: fixed; left: 0; top: 0; width: 800px; height: 600px; border: 0"></iframe>`)
	}))
	defer vscode.Close()
	var r report
	got := s.clip(t, func() {
		b.navigate(vscode.URL)
		r = b.pressInFrame("document.getElementById('frame')")
	})
	if got != copied["code"] {
		t.Errorf("copies %q, want %q", got, copied["code"])
	}
	if r.Status != "Copied" || !r.Focused || r.Shown {
		t.Errorf("after the press the status is %q, the button focused %v, its focus shown %v; want Copied, true, false", r.Status, r.Focused, r.Shown)
	}
}

// TestUICopyNoClipboard presses the copy button of code on a page without navigator.clipboard, as one that is no
// secure context: the copy command copies.
func TestUICopyNoClipboard(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	got := s.clip(t, func() {
		b.navigate("/a/copy/?raw")
		b.eval("Object.defineProperty(navigator, 'clipboard', { value: undefined })")
		b.pressButton("code")
		b.until("the copy", status("code")+" === 'Copied'")
	})
	if got != copied["code"] {
		t.Errorf("copies %q, want %q", got, copied["code"])
	}
}

// TestUICopyFile presses the copy button of code on the page floe render writes, opened as a file.
func TestUICopyFile(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	file := s.write(t, "file/copy.yaml", fmt.Sprintf(copyPage, "en", longPath))
	s.floe(t, "render", file)
	got := s.clip(t, func() {
		b.navigate("file://" + strings.TrimSuffix(file, ".yaml") + ".html")
		b.until("the copy button", button("code"))
		b.gesture(button("code") + ".click()")
		b.until("the copy", status("code")+" === 'Copied'")
	})
	if got != copied["code"] {
		t.Errorf("copies %q, want %q", got, copied["code"])
	}
}

// TestUICopyFails presses the copy button of code where neither the clipboard nor the copy command writes, the
// command failing or throwing: the button says it could not copy, and shows no check.
func TestUICopyFails(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	for _, command := range []string{"() => false", "() => { throw new Error('no'); }"} {
		b.navigate("/a/copy/?raw")
		b.eval("navigator.clipboard.writeText = () => Promise.reject(new Error('no')); document.execCommand = " + command)
		b.pressButton("code")
		got := b.until("the failure", status("code")+" && "+status("code")+" + '|' + "+button("code")+".getAttribute('aria-label') + '|' + "+
			button("code")+".classList.contains('copied')")
		if want := "Could not copy|Could not copy|false"; got != want {
			t.Errorf("with the command %s: %s, want %s", command, got, want)
		}
	}
}

// TestUICopyWords draws the button in the page's language.
func TestUICopyWords(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "ja")
	b := s.b
	b.navigate("/a/copy/?raw")
	b.until("the copy button", button("code"))
	label := b.eval(button("code") + ".getAttribute('aria-label')")
	b.pressButton("code")
	got := b.until("the copy", status("code"))
	if label != "コピー" || got != "コピーしました" {
		t.Errorf("the label is %q and the status %q, want コピー and コピーしました", label, got)
	}
}

// snipKit is a part of one's own, snip, that does not take code's look and puts its copy button in a box of its
// own, and a template that places it.
var snipKit = map[string]string{
	"parts/snip/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
	"parts/snip/part.html":   `<div data-floe-part="snip"><p><code data-floe-copy>x</code><span>{{copyButton}}</span></p></div>`,
	"parts/snip/part.css":    "",
	"templates/snips/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{.Data.snip}}\n</body></html>\n",
	"templates/snips/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"snip": {"$ref": "floe:///parts/snip.json"}}}`,
	"templates/snips/example.yaml": "lang: en\ntemplate: snips\nsnip: {}\n",
}

// TestUICopyOwnPart presses the copy button a part of one's own draws: it looks as in floe's parts, one icon and its
// status out of sight, and copies the part's marked text from inside a box of its own.
func TestUICopyOwnPart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := copySite(t, "en")
	b := s.b
	for name, text := range snipKit {
		at := filepath.Join(s.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "publish", s.write(t, "snip.yaml", "lang: en\ntemplate: snips\ntitle: Snip\nsnip: {}\n"), "--slug", "snip")
	var look string
	got := s.clip(t, func() {
		b.navigate("/a/snip/?raw")
		b.until("the copy button", button("snip"))
		look = b.eval(`(() => {
			const button = ` + button("snip") + `, r = button.getBoundingClientRect();
			const icons = [...button.querySelectorAll('svg')].filter((i) => getComputedStyle(i).display !== 'none').length;
			return [Math.round(r.width), Math.round(r.height), icons].join();
		})()`)
		b.pressButton("snip")
		b.until("the copy", status("snip")+" === 'Copied'")
		look += "," + b.eval("(({ width, height }) => width <= 1 && height <= 1)(document.querySelector('.floe-copy-status').getBoundingClientRect())")
	})
	if want := "30,30,1,true"; look != want {
		t.Errorf("the button of snip is %s, want %s (30 px wide and high, one icon, its status out of sight once it speaks)", look, want)
	}
	if got != "x\n" {
		t.Errorf("copies %q, want %q", got, "x\n")
	}
}
