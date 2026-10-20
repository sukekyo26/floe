package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
)

// A publish larger than the server takes is told so in the user's language, with the limit.
func TestTooLarge(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
	}))
	t.Cleanup(srv.Close)
	err := client{base: srv.URL, lang: i18n.English}.do(t.Context(), http.MethodPut, "/api/artifacts/p", struct{}{}, nil)
	if err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Errorf("a refusal for its size = %v, want the limit told", err)
	}
}
