package cli

import (
	"io"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// with is an app whose environment is env.
func with(env map[string]string) *app {
	return &app{env: Env{Stdout: io.Discard, Stderr: io.Discard, Dir: "/", Getenv: func(k string) string { return env[k] }}, out: io.Discard}
}

// The server's settings are read as a command reads them: no place for the icons is none (""), as a command
// draws then, the same for floe serve and the command that finds it; any other mistake stops it.
func TestServerConfig(t *testing.T) {
	t.Parallel()

	cfg, err := with(map[string]string{"FLOE_DIR": "/s", "HOME": "rel"}).serverConfig()
	if err != nil || cfg.Icons != "" || cfg.Store.Root() != "/s" || cfg.Port != 7325 {
		t.Errorf("serverConfig = %+v, %v", cfg, err)
	}
	cfg, err = with(map[string]string{"HOME": "/h", "FLOE_PORT": "8000"}).serverConfig()
	if err != nil || cfg.Store.Root() != "/h/.local/share/floe" || cfg.Icons != "/h/.local/share/floe-icons" ||
		cfg.Parts != "/h/.config/floe/parts" || cfg.Port != 8000 || cfg.Home != "/h" {
		t.Errorf("serverConfig = %+v, %v", cfg, err)
	}
	for _, tt := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, "floe: there is no directory for pages: HOME is not an absolute path, or the default one is under a file; set FLOE_DIR to an absolute path"},
		{map[string]string{"HOME": "/h", "FLOE_PORT": "0x10"}, `floe: invalid FLOE_PORT "0x10": use a port number from 1 to 65535`},
		{map[string]string{"HOME": "/h", "FLOE_PORT": "70000"}, `floe: invalid FLOE_PORT "70000": use a port number from 1 to 65535`},
		{map[string]string{"HOME": "/h", "FLOE_PORT": "+80"}, `floe: invalid FLOE_PORT "+80": use a port number from 1 to 65535`},
		{map[string]string{"HOME": "/h", "FLOE_ICONS_DIR": "rel"}, "floe: FLOE_ICONS_DIR is rel, a relative path; set it to an absolute path"},
	} {
		_, err := with(tt.env).serverConfig()
		if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != tt.want {
			t.Errorf("%v: told\n%s\nwant\n%s", tt.env, got, tt.want)
		}
	}
}

// A server a command starts takes the settings the command read, one unset there unset too.
func TestServerEnv(t *testing.T) {
	t.Setenv("FLOE_PORT", "1234")
	t.Setenv("FLOE_PARTS_DIR", "/elsewhere")
	env := with(map[string]string{"FLOE_PORT": "8000", "HOME": "/h"}).serverEnv()
	seen := map[string]bool{}
	for _, kv := range env {
		seen[kv] = true
	}
	if !seen["FLOE_PORT=8000"] || !seen["HOME=/h"] || seen["FLOE_PORT=1234"] || seen["FLOE_PARTS_DIR=/elsewhere"] {
		t.Errorf("env %v", env)
	}
}
