package browsertest_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// copyPage is a page of lang with a part of each kind that copies: code ending in a line break, a terminal whose
// last command ends in one, and a diff that does not. Its script reports the copy button of the code, as the
// center of its box, its status, and whether it has the focus and shows it, to a frame around it.
const copyPage = `template: report
lang: %s
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
      - diff: { file: a.go, text: "@@ -1,2 +1,2 @@\n package a\n-var x = 1\n+var x = 2" }
      - html: |
          <script>
          document.addEventListener('pointerover', () => parent.postMessage('over', '*'));
          setInterval(() => {
            const button = document.querySelector('[data-floe-part=code] .copy');
            const r = button.getBoundingClientRect();
            parent.postMessage(JSON.stringify({
              x: r.x + r.width / 2, y: r.y + r.height / 2,
              status: document.querySelector('[data-floe-part=code] .copy-status').textContent,
              focused: document.activeElement === button, shown: button.matches(':focus-visible'),
            }), '*');
          }, 100);
          </script>
`

// copied is what the button of each part copies.
var copied = map[string]string{
	"code":     "package main\n\nfunc main() {}\n",
	"terminal": "go test ./...\ngit commit \\\n  -m \"fix\"\necho one\n",
	"diff":     "@@ -1,2 +1,2 @@\n package a\n-var x = 1\n+var x = 2\n",
}

// copySite is a site with the page of copyPage in lang published as copy, whose tab may read the clipboard on
// the server's origin and is always focused, as the clipboard asks.
func copySite(t *testing.T, lang string) *site {
	t.Helper()

	s := newSite(t)
	s.floe(t, "publish", s.write(t, "copy.yaml", fmt.Sprintf(copyPage, lang)), "--slug", "copy")
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
	return "document.querySelector('[data-floe-part=" + part + "] .copy')"
}

// status is the status of the copy button of part on the page shown.
func status(part string) string {
	return "document.querySelector('[data-floe-part=" + part + "] .copy-status').textContent"
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
				const button = ` + button(part) + `, part = button.parentElement, pre = part.querySelector('pre');
				const style = getComputedStyle(button), ground = getComputedStyle(pre).backgroundColor;
				const inside = (a, b) => a.left >= b.left && a.right <= b.right && a.top >= b.top && a.bottom <= b.bottom;
				return [
					document.activeElement === button && button.matches(':focus-visible'),
					style.outlineStyle, style.outlineColor === getComputedStyle(pre).color, parseFloat(style.outlineOffset) < 0,
					style.backgroundColor === (ground === 'rgba(0, 0, 0, 0)' ? getComputedStyle(part).backgroundColor : ground),
					inside(button.getBoundingClientRect(), pre.getBoundingClientRect()), button.getAttribute('aria-label'),
				].join();
			})()`)
			b.pressKey(kb.Enter)
			b.until("the copy of "+part+" by the keyboard", status(part)+" === 'Copied'")
		})
		if got != copied[part] {
			t.Errorf("%s by the keyboard copies %q, want %q", part, got, copied[part])
		}
		if want := "true,solid,true,true,true,true,Copy"; look != want {
			t.Errorf("%s: the focused button is %s, want %s (focus shown, ring, its colour the code's, inside, ground the code's, in the code, label)", part, look, want)
		}
	}

	// After 1.5 seconds the button is as before; a second press starts the 1.5 seconds again.
	b.navigate("/a/copy/?raw")
	b.pressButton("code")
	b.until("the copy", status("code")+" === 'Copied' && "+button("code")+".classList.contains('copied')")
	b.until("the button to come back", status("code")+" === '' && "+button("code")+".getAttribute('aria-label') === 'Copy' && "+
		button("code")+".title === 'Copy' && !"+button("code")+".classList.contains('copied')")
	b.pressButton("code")
	time.Sleep(time.Second)
	b.pressButton("code")
	time.Sleep(800 * time.Millisecond)
	if got := b.eval(status("code")); got != "Copied" {
		t.Errorf("0.8 s after a second press, 1.8 s after the first, the status is %q, want Copied", got)
	}

	// A press whose copy ends after a later press's shows nothing: the later one's 1.5 seconds hold.
	b.navigate("/a/copy/?raw")
	b.eval("{ let n = 0; navigator.clipboard.writeText = () => new Promise((done) => setTimeout(done, n++ === 0 ? 2000 : 0)); }")
	b.pressButton("code")
	time.Sleep(100 * time.Millisecond)
	b.pressButton("code")
	time.Sleep(2400 * time.Millisecond)
	if got := b.eval(status("code") + " + '|' + " + button("code") + ".getAttribute('aria-label')"); got != "|Copy" {
		t.Errorf("2.5 s after a press whose copy took 2 s, and 2.4 s after a second, the status and label are %q, want |Copy", got)
	}
}

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
	file := s.write(t, "file/copy.yaml", fmt.Sprintf(copyPage, "en"))
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
