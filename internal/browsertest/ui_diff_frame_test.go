package browsertest_test

import (
	"strings"
	"testing"
)

// TestUIDiffFrame opens what changed in the marked page's frame: n moves the frame to a place and leaves the URL of
// the diff as it is, and a version published while it is open reads what changed again, with the frame's page of the
// key the answer gives.
func TestUIDiffFrame(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	b.navigate("/a/d/?diff&from=2&to=3")
	const shown = "typeof diffAnswer !== 'undefined' && diffAnswer !== null && document.getElementById('diff-frame').src.includes('key=' + diffAnswer.key)"
	b.until("the marked page", shown)
	out := make([]string, 0, 4)
	b.pressKey("n")
	out = append(out, b.until("the frame at the first place", "document.getElementById('diff-frame').src.endsWith('#floe-diff.0') && location.search + '|' + location.hash"))
	// Wait for the page in the frame to tell where it is, which it does as it moves: the URL stays.
	b.still()
	out = append(out, b.eval("location.search + '|' + location.hash"))
	key := b.eval("diffAnswer.key")
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>One more.</p>"), "--slug", "d")
	out = append(out, b.until("what changed read again", "diffAnswer.key !== '"+key+"' && ("+shown+") && String(diffVersions.length)"))
	check(t, "frame", out, strings.Join([]string{"?diff&from=2&to=3|", "?diff&from=2&to=3|", "4"}, "|"))
}
