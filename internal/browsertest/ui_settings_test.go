package browsertest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUISettings opens the settings from the bar's button, shown at the width of a desk, not of a phone, the bar within
// the window at both, shows a card of each theme with its sample drawn in the sandbox, and writes the default theme chosen
// from them, which a new page is then drawn in; a settings.json that cannot be read is said, and choosing mends it.
func TestUISettings(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	out := make([]string, 0, 16)
	for _, width := range []int64{1440, 390} {
		b.navigate("/")
		b.size(width, 900, width < 600)
		// Shown where the bar has room for it, hidden at 40rem or less; the bar within the window either way
		out = append(out, b.eval(`(() => {
			const r = document.getElementById('settings-open').getBoundingClientRect();
			return [r.width > 0 && r.right <= innerWidth, document.documentElement.scrollWidth <= innerWidth].join(' ');
		})()`))
	}
	b.size(1440, 900, false)
	b.eval("document.getElementById('settings-open').click()")
	b.until("the cards", "document.querySelectorAll('#theme-grid .theme-card').length >= 12")
	out = append(out,
		b.eval("document.getElementById('settings').open"),
		b.eval("document.getElementById('theme-now').textContent"),
		// default is the one now, picked; each card's sample in the sandbox, from the server
		b.eval("document.querySelector('#theme-grid input:checked').value"),
		b.eval("[...document.querySelectorAll('#theme-grid iframe')].every((f) => f.getAttribute('sandbox') === '' && f.src.endsWith('/sample'))"),
		b.eval("document.querySelector('#theme-grid input[value=ink]').closest('label').querySelector('.ds').textContent"),
	)
	b.eval("document.querySelector('#theme-grid input[value=ink]').click(); document.getElementById('theme-choose').click()")
	b.until("the toast", "[...document.querySelectorAll('.toast')].some((e) => e.textContent.includes('Changed the default theme to ink'))")
	out = append(out, b.eval("document.getElementById('settings').open"), strings.TrimSpace(s.floe(t, "theme", "default")))
	// A page stored now is drawn in ink.
	s.floe(t, "publish", s.page(t, "new.yaml", "New", ""))
	out = append(out, b.eval("fetch('/a/new/?raw').then((r) => r.text()).then((t) => t.includes('--accent: #2b3a67'))"))
	// settings.json broken: said, and mended by choosing.
	if err := os.WriteFile(filepath.Join(s.Env["FLOE_DIR"], "settings.json"), []byte(`{"theme": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b.eval("document.getElementById('settings-open').click()")
	b.until("the cards again", "document.getElementById('settings').open && document.querySelectorAll('#theme-grid .theme-card').length >= 12")
	out = append(out, b.eval("document.getElementById('theme-now').textContent"))
	b.eval("document.querySelector('#theme-grid input[value=washi]').click(); document.getElementById('theme-choose').click()")
	b.until("mended", "!document.getElementById('settings').open")
	out = append(out, strings.TrimSpace(s.floe(t, "theme", "default")))
	check(t, "settings", out, "true true|false true|true|Default now: default. Pages stored from now on are drawn in this theme. The pages there are now do not change.|default|true|Ink on paper: near black, an indigo ink for emphasis, its sections ruled in the ink|false|ink|true|Failed to read the settings. Choosing a theme writes them again|washi")
}
