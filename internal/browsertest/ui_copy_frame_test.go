package browsertest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
