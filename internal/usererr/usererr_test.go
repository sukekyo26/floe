package usererr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

func TestError(t *testing.T) {
	t.Parallel()

	if got := usererr.Library(errors.New("bad")).Error(); got != "bad" {
		t.Errorf("Error() = %q", got)
	}
}

// Tell words a mistake, one wrapped in another error, and floe's failure; Render lays them out to the letter
// as the command line and the browser show them, floe's name before each.
func TestTell(t *testing.T) {
	t.Parallel()

	lang := usererr.Library(errors.New("no lang"))
	for _, tt := range []struct {
		name        string
		err         error
		text, named string
	}{
		{"one", lang, "no lang", "floe: no lang"},
		{"one wrapped, without the words of what wraps it", fmt.Errorf("read: %w", lang), "no lang", "floe: no lang"},
		{"no mistake", (*usererr.Error)(nil), "*usererr.Error with no mistake", "floe: *usererr.Error with no mistake"},
		{"none", nil, "<nil> with no mistake", "floe: <nil> with no mistake"},
		{"floe's failure", errors.New("disk on fire"), "disk on fire", "floe: disk on fire"},
		{"floe's failure without words", errors.New(""), "*errors.errorString", "floe: *errors.errorString"},
	} {
		r := usererr.Tell(i18n.EnglishCatalog(), tt.err)
		if got := r.Render(""); got != tt.text {
			t.Errorf("%s: Render(\"\") = %q, want %q", tt.name, got, tt.text)
		}
		if got := r.Render("floe: "); got != tt.named {
			t.Errorf("%s: Render(\"floe: \") = %q, want %q", tt.name, got, tt.named)
		}
	}
	// A library's words stay as it said them in every language.
	if got := usererr.Tell(i18n.New("ja"), lang).Render(""); got != "no lang" {
		t.Errorf("Tell(ja) = %q", got)
	}
}
