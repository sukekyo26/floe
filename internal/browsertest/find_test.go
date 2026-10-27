package browsertest_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Chrome is FLOE_CHROME, else found on PATH, else none and the tests skipped; but on CI, FLOE_CHROME is required,
// so that a lost pin fails rather than runs the runner's Chrome.
func TestFindChrome(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	there, unrun := filepath.Join(dir, "chrome"), filepath.Join(dir, "chrome.txt")
	for _, file := range []string{there, unrun} {
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(there, 0o700); err != nil { //nolint:gosec // G302: a program to run
		t.Fatal(err)
	}
	onPath := func(name string) (string, error) {
		if name == "chromium" {
			return "/usr/bin/chromium", nil
		}
		return "", exec.ErrNotFound
	}
	nowhere := func(string) (string, error) { return "", exec.ErrNotFound }
	for _, c := range []struct {
		env      map[string]string
		lookPath func(string) (string, error)
		path     string
		skip     bool
		err      bool
	}{
		{map[string]string{"FLOE_CHROME": there}, nowhere, there, false, false},
		{map[string]string{"FLOE_CHROME": there, "CI": "true"}, onPath, there, false, false},
		{map[string]string{"FLOE_CHROME": "/none/chrome"}, onPath, "", false, true},
		{map[string]string{"FLOE_CHROME": dir}, onPath, "", false, true},
		{map[string]string{"FLOE_CHROME": unrun}, onPath, "", false, true},
		{map[string]string{}, onPath, "/usr/bin/chromium", false, false},
		{map[string]string{}, nowhere, "", true, false},
		{map[string]string{"CI": "true"}, onPath, "", false, true},
		{map[string]string{"CI": "true"}, nowhere, "", false, true},
	} {
		path, skip, err := findChrome(func(k string) string { return c.env[k] }, c.lookPath)
		if path != c.path || skip != c.skip || (err != nil) != c.err || err != nil && !errors.Is(err, errNoChrome) {
			t.Errorf("%v: %q %v %v, want %q %v and an error %v", c.env, path, skip, err, c.path, c.skip, c.err)
		}
	}
}

// httpGet answers url, read whole.
func httpGet(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err //nolint:wrapcheck // the test's failure
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err //nolint:wrapcheck // as above
	}
	defer res.Body.Close() //nolint:errcheck // read only
	b, err := io.ReadAll(res.Body)
	return string(b), err //nolint:wrapcheck // as above
}
