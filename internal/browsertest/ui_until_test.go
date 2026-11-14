package browsertest_test

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// A page moved to whole has its new URL while it still loads, before its script runs: a wait for the URL
// must not answer until the script has run.
func TestUIUntilLoaded(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.navigate("/a/d/")
	// A slow CPU, as a loaded machine, keeps the new page loading long enough to be read.
	slow := chromedp.Func(func(ctx context.Context, tg *chromedp.Target) error {
		_, err := cdp.Call(ctx, tg, emulation.SetCPUThrottlingRate, emulation.SetCPUThrottlingRateParams{Rate: 20})
		return err //nolint:wrapcheck // the test's to tell
	})
	if _, err := chromedp.Run(b.ctx, slow); err != nil {
		t.Fatalf("slow the CPU: %v", err)
	}
	b.moving("location.href = '/a/one/'")
	b.until("the page", "location.pathname === '/a/one/'")
	if got := b.eval("typeof ORDER_KEY"); got != "string" {
		t.Errorf("the page's script after the wait: typeof ORDER_KEY = %q, want string", got)
	}
}
