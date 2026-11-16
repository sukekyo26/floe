//go:build windows

package cli

import (
	"errors"
	"path/filepath"
	"testing"

	"floe/internal/usererr"
)

// homeVars is the environment of a home at home and no other setting: on Windows, the profile and its
// AppData\Local, as the system sets them.
func homeVars(home string) map[string]string {
	return map[string]string{"USERPROFILE": home, "LOCALAPPDATA": filepath.Join(home, "AppData", "Local")}
}

// defaultPlace is where floe keeps what (store, icons, parts, themes, templates) by default in the home of homeVars,
// as Windows has it: all under %LOCALAPPDATA%\floe.
func defaultPlace(home, what string) string {
	local := filepath.Join(home, "AppData", "Local", "floe")
	if what == "store" {
		return filepath.Join(local, "pages")
	}
	return filepath.Join(local, what)
}

// placeVar is the variable a mistake of no default place of what names (store, icons, parts, …, skills).
func placeVar(what string) string {
	if what == "skills" {
		return "USERPROFILE"
	}
	return "LOCALAPPDATA"
}

// Windows' default places come from LOCALAPPDATA alone: HOME, the XDG base directories and the profile do not stand in
// for it, a relative one is none, and a setting is still an absolute path.
func TestWindowsPlaces(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	for name, env := range map[string]map[string]string{
		"HOME, XDG and the profile":   {"HOME": home, "XDG_DATA_HOME": home, "XDG_CONFIG_HOME": home, "USERPROFILE": home},
		"LOCALAPPDATA relative":       {"USERPROFILE": home, "LOCALAPPDATA": `AppData\Local`},
		"LOCALAPPDATA drive-relative": {"USERPROFILE": home, "LOCALAPPDATA": `C:AppData`},
	} {
		a := with(env)
		if _, err := a.storeDir(); !isKey(err, "store_no_dir") {
			t.Errorf("%s: storeDir = %v, want store_no_dir", name, err)
		}
		for _, kind := range []string{"parts", "themes", "templates"} {
			if p, err := a.ownPlace(kind); err != nil || p.dir != "" || p.from != "LOCALAPPDATA" {
				t.Errorf("%s: ownPlace(%s) = %+v, %v, want none", name, kind, p, err)
			}
		}
	}
	if _, err := with(map[string]string{"FLOE_DIR": `floe\pages`}).storeDir(); !isKey(err, "dir_relative") {
		t.Errorf("a relative FLOE_DIR: storeDir = %v, want dir_relative", err)
	}
	if _, err := with(map[string]string{"FLOE_DIR": `\floe`}).storeDir(); !isKey(err, "dir_relative") {
		t.Errorf("a FLOE_DIR of no drive: storeDir = %v, want dir_relative", err)
	}
}

// isKey reports whether err is the mistake of key.
func isKey(err error, key string) bool {
	e, ok := errors.AsType[*usererr.Error](err)
	return ok && e.Key == key
}
