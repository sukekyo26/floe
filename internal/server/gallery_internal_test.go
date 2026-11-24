package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The management page's gallery serves a page in the sandbox a page of floe is (internal/ui/web/vite.config.ts), so
// that what the frame takes of a page's messages is tried as it is: the same words as pageCSP.
func TestGalleryPageSandbox(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile(filepath.Join("..", "ui", "web", "vite.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "'"+pageCSP+"'") {
		t.Errorf("vite.config.ts has no PAGE_CSP of %q: write pageCSP's words there", pageCSP)
	}
}
