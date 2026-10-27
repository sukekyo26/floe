package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// A page made a FIFO after drawPages looked at it is refused unopened as it is written or checked, and left as it is.
func TestPageMadeFIFO(t *testing.T) {
	t.Parallel()

	fifo := filepath.Join(t.TempDir(), "a.html")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	pages := []rendered{{from: "a.yaml", to: "a.html", real: fifo, page: []byte("page")}}
	want := "a.html: this is neither a regular file nor a directory (a FIFO, a device or the like), which floe may never finish reading: put a regular file here"
	var out bytes.Buffer
	a := &app{env: Env{Stdout: &out, Stderr: nil, Dir: "", Getenv: nil}, out: &out}
	for name, err := range map[string]error{"writePages": a.writePages(pages, arg{name: "", abs: "", err: nil}), "checkPages": checkPages(pages)} {
		if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != want {
			t.Errorf("%s told %q, want %q", name, got, want)
		}
	}
	if info, err := os.Lstat(fifo); err != nil || info.Mode()&os.ModeNamedPipe == 0 || strings.Contains(out.String(), "a.html") {
		t.Errorf("the FIFO is now %v, %v; printed %q", info, err, out.String())
	}
}
