package cli

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/server"
)

// A publish of a data file larger than the server takes is told so with the limit, whether the server
// refuses it or it is too large to send; a refusal for its size of anything else is left as it is.
func TestTooLarge(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
	}))
	t.Cleanup(srv.Close)
	c := client{base: srv.URL}
	refused := c.do(t.Context(), http.MethodPut, "/api/artifacts/p", struct{}{}, nil)
	if err := tooLarge(refused); err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Errorf("a publish refused for its size = %v, want the limit told", err)
	}
	if re, ok := errors.AsType[*remoteError](refused); !ok || re.status != http.StatusRequestEntityTooLarge ||
		strings.Contains(re.Error(), "32 MiB") {
		t.Errorf("a refusal for its size of the client = %v, want it as the server sent it", refused)
	}
	// A refusal without a reason says what the server answered.
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(empty.Close)
	if err := (client{base: empty.URL}).do(t.Context(), http.MethodGet, "/api/artifacts", nil, nil); err == nil ||
		err.Error() != "502 Bad Gateway" {
		t.Errorf("a refusal without a reason = %v", err)
	}
	// Nor does a report of no mistake (from a proxy, say) go out as an empty line.
	none := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"mistakes":[]}`)) //nolint:errcheck // the test reads what arrives
	}))
	t.Cleanup(none.Close)
	if err := (client{base: none.URL}).do(t.Context(), http.MethodGet, "/api/artifacts", nil, nil); err == nil ||
		err.Error() != `{"mistakes":[]}` {
		t.Errorf("a report of no mistake = %v, want the answer as sent", err)
	}
	large := map[string]string{"source": strings.Repeat("x", server.MaxUpload)}
	unsent := client{base: "http://127.0.0.1:1"}.do(t.Context(), http.MethodPut, "/api/artifacts/p", large, nil)
	if err := tooLarge(unsent); err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Errorf("a publish over the limit = %v, want the limit told before it is sent", err)
	}
}

// Every image the server asks for that cannot be sent is told at once; one not there is left to the server.
func TestReadImages(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png"} {
		if err := os.Symlink(filepath.Join(dir, "nowhere"), filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	_, err := readImages(dir, []string{"a.png", "none.png", "b.png"})
	lines := strings.Split(fmt.Sprint(err), "\n")
	if err == nil || len(lines) != 2 || !strings.HasPrefix(lines[0], `"a.png" is reached through a symlink`) ||
		!strings.HasPrefix(lines[1], `"b.png" is reached through a symlink`) {
		t.Errorf("readImages of two symlinks = %v, want both", err)
	}
}
