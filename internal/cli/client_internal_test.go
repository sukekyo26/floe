package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/server"
)

// A publish larger than the server takes is told so in the user's language, with the limit, whether the
// server refuses it or it is too large to send.
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
	// One over the limit is told before it is sent.
	large := map[string]string{"source": strings.Repeat("x", server.MaxUpload)}
	err = client{base: "http://127.0.0.1:1", lang: i18n.English}.do(t.Context(), http.MethodPut, "/api/artifacts/p", large, nil)
	if err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Errorf("a publish over the limit = %v, want the limit told", err)
	}
}
