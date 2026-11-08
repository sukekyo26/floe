package browsertest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Another site cannot frame the management page (Chrome shows its error page in the frame instead),
// while a page that moves its own frame to the management page shows it nested.
func TestUIFraming(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	out := make([]string, 0, 2)

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<iframe src="`+s.Base+`/"></iframe>`) //nolint:errcheck // the test checks the frame
	}))
	defer other.Close()
	if err := chromedp.Do(b.ctx, chromedp.Navigate(other.URL)); err != nil {
		t.Fatal(err)
	}
	framed := ""
	for deadline := time.Now().Add(waitLimit); framed == "" && time.Now().Before(deadline); {
		framed = framedDocument(t, b)
		time.Sleep(50 * time.Millisecond)
	}
	out = append(out, "framed:"+framed)

	s.floe(t, "publish", "--link", s.write(t, "site/mover.html", "<script>location.href = '/'</script>"), "--slug", "mover")
	b.navigate("/a/mover/")
	out = append(out, b.until("the management page in the frame", "frames[0].document.title"))

	check(t, "framing", out, "framed:chrome-error://chromewebdata/|floe")
}
