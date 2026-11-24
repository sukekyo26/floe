package store

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// taskURLCase is one URL of testdata/task-urls.json, which the management page's add modal reads too (urlLink): what the
// store says of it, what the modal says, and, where they differ, the side the modal takes: lenient (it adds what the
// store refuses as it saves) or strict (it refuses what the browser cannot open as a link).
type taskURLCase struct {
	URL     string `json:"url"`
	Store   bool   `json:"store"`
	Modal   bool   `json:"modal"`
	Differs string `json:"differs"`
}

// The store's reading of the URLs the modal is checked against: a change of either side shows here or in its test.
func TestValidURLCases(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("testdata/task-urls.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []taskURLCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	want := map[bool]string{true: "lenient", false: "strict"}
	for _, c := range cases {
		if got := validURL(strings.TrimSpace(c.URL)); got != c.Store {
			t.Errorf("%q: validURL is %v, want %v", c.URL, got, c.Store)
		}
		if c.Store == c.Modal && c.Differs != "" || c.Store != c.Modal && c.Differs != want[c.Modal] {
			t.Errorf("%q: differs is %q, as the store says %v and the modal %v", c.URL, c.Differs, c.Store, c.Modal)
		}
	}
}
