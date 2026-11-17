package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/link"
	"floe/internal/usererr"
)

// with is an app whose environment is env.
func with(env map[string]string) *app {
	return &app{env: Env{Stdout: io.Discard, Stderr: io.Discard, Dir: "/", Getenv: func(k string) string { return env[k] }}, out: io.Discard}
}

// The server's settings are read as a command reads them: no place for the icons is none (""), as a command
// draws then, the same for floe serve and the command that finds it; any other mistake stops it.
func TestServerConfig(t *testing.T) {
	t.Parallel()

	t.Run("the places", func(t *testing.T) {
		t.Parallel()
		store, home := filepath.Join(t.TempDir(), "s"), t.TempDir()
		cfg, _, err := with(map[string]string{"FLOE_DIR": store, link.HomeKey: "rel"}).serverConfig()
		if err != nil || cfg.Icons != "" || cfg.Store.Root() != store || cfg.Port != 7325 {
			t.Errorf("serverConfig = %+v, %v", cfg, err)
		}
		cfg, _, err = with(HomeEnv(home, map[string]string{"FLOE_PORT": "8000"})).serverConfig()
		if err != nil || cfg.Store.Root() != defaultPlace(home, "store") || cfg.Icons != defaultPlace(home, "icons") ||
			cfg.Parts != defaultPlace(home, "parts") || cfg.Port != 8000 || cfg.Home != home {
			t.Errorf("serverConfig = %+v, %v", cfg, err)
		}
		// A default under a file is none to read, as a command reads it.
		file := filepath.Join(t.TempDir(), "file")
		if werr := os.WriteFile(file, nil, 0o600); werr != nil {
			t.Fatal(werr)
		}
		cfg, _, err = with(HomeEnv(file, map[string]string{"FLOE_DIR": store})).serverConfig()
		if err != nil || cfg.Icons != "" || cfg.Parts != "" || cfg.Themes != "" || cfg.Templates != "" {
			t.Errorf("serverConfig under a file = %+v, %v", cfg, err)
		}
	})
	home := t.TempDir()
	for name, tt := range map[string]struct {
		env  map[string]string
		want string
	}{
		"no home":            {map[string]string{}, "floe: there is no directory for pages, as " + placeVar("store") + " is not an absolute path; set FLOE_DIR to an absolute path"},
		"port 0x10":          {HomeEnv(home, map[string]string{"FLOE_PORT": "0x10"}), `floe: invalid FLOE_PORT "0x10": use a port number from 1 to 65535`},
		"port 70000":         {HomeEnv(home, map[string]string{"FLOE_PORT": "70000"}), `floe: invalid FLOE_PORT "70000": use a port number from 1 to 65535`},
		"port +80":           {HomeEnv(home, map[string]string{"FLOE_PORT": "+80"}), `floe: invalid FLOE_PORT "+80": use a port number from 1 to 65535`},
		"icons dir relative": {HomeEnv(home, map[string]string{"FLOE_ICONS_DIR": "rel"}), "floe: FLOE_ICONS_DIR is rel, a relative path; set it to an absolute path"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, err := with(tt.env).serverConfig()
			if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != tt.want {
				t.Errorf("%v: told\n%s\nwant\n%s", tt.env, got, tt.want)
			}
		})
	}
}

// A mistake of no default place names the variable of the system that decides it.
func TestNoDirTellsTheVariable(t *testing.T) {
	t.Parallel()

	english := i18n.EnglishCatalog()
	none := with(map[string]string{})
	_, err := none.storeDir()
	_, iconsErr := none.iconsDir()
	for what, c := range map[string]struct {
		err  error
		want string
	}{
		"store": {err, "there is no directory for pages, as " + placeVar("store") + " is not an absolute path"},
		"icons": {iconsErr, "there is no directory for icons, as " + placeVar("icons") + " is not an absolute path"},
	} {
		if got := usererr.Tell(english, c.err).Render(""); !strings.HasPrefix(got, c.want) {
			t.Errorf("%s: told %q, want it to begin %q", what, got, c.want)
		}
	}
	p, err := none.ownPlace("parts")
	if err != nil || p.dir != "" || p.from != placeVar("parts") {
		t.Errorf("ownPlace(parts) of no home = %+v, %v", p, err)
	}
	p, _, err = none.skillDir(agents[0])
	if got := usererr.Tell(english, err).Render(""); p.dir != "" || got != "there is no default directory, as "+placeVar("skills")+
		" is not an absolute path; set "+link.HomeKey+" to an absolute path" {
		t.Errorf("skillDir of no home told %q", got)
	}
}

// A server a command starts takes the settings the command read, one unset there unset too: the home and the base of
// the data too, whatever the process has.
func TestServerEnv(t *testing.T) {
	t.Setenv("FLOE_PORT", "1234")
	t.Setenv("FLOE_PARTS_DIR", "/elsewhere")
	t.Setenv(dataBase.key, "/process")
	home := t.TempDir()
	env := with(HomeEnv(home, map[string]string{"FLOE_PORT": "8000", dataBase.key: home})).serverEnv()
	seen := map[string]bool{}
	for _, kv := range env {
		seen[kv] = true
	}
	if !seen["FLOE_PORT=8000"] || !seen[link.HomeKey+"="+home] || !seen[dataBase.key+"="+home] || seen["FLOE_PORT=1234"] ||
		seen["FLOE_PARTS_DIR=/elsewhere"] || seen[dataBase.key+"=/process"] {
		t.Errorf("env %v", env)
	}
}
