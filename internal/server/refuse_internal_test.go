package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"floe/internal/store"
)

// What the store says is not there is 404 however it is wrapped, whoever asks: the mistake of a page gone between two
// looks (edit) comes so.
func TestRefuseNotThere(t *testing.T) {
	t.Parallel()

	for _, err := range []error{store.ErrNotFound, fmt.Errorf("edit: %w", store.ErrNotFound)} {
		for _, accept := range []string{"", jsonType} {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/api/pages/d", nil)
			r.Header.Set("Accept", accept)
			w := httptest.NewRecorder()
			refuse(w, r, err)
			if w.Code != http.StatusNotFound || w.Body.String() != "not found" {
				t.Errorf("%v, Accept %q: %d %q", err, accept, w.Code, w.Body)
			}
		}
	}
}
