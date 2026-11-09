package cli_test

import (
	"os"
	"path/filepath"
	"testing"
)

// floe theme default prints the default theme chosen, default too, nothing for none, reading settings.json of the
// store without a server (none answers on the port named); one it cannot read is its mistake.
func TestThemeDefaultCommand(t *testing.T) {
	t.Parallel()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(tmp, "store")
	env := map[string]string{"HOME": tmp, "FLOE_DIR": store, "FLOE_PORT": "1"}
	if code, out, errs := runEnv(tmp, env, "theme", "default"); code != 0 || out != "" || errs != "" {
		t.Errorf("none chosen: %d %q %q", code, out, errs)
	}
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	for written, want := range map[string]string{`{"theme": "default"}`: "default\n", `{"theme": "ink"}`: "ink\n", `{}`: ""} {
		if err := os.WriteFile(filepath.Join(store, "settings.json"), []byte(written), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, out, errs := runEnv(tmp, env, "theme", "default"); code != 0 || out != want || errs != "" {
			t.Errorf("%s: %d %q %q", written, code, out, errs)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "settings.json"), []byte(`{"theme": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(store, "settings.json") + `: json: cannot unmarshal JSON number into Go string within "/theme"` + "\n"
	if code, out, errs := runEnv(tmp, env, "theme", "default"); code != 1 || out != "" || errs != want {
		t.Errorf("broken: %d %q %q", code, out, errs)
	}
}
