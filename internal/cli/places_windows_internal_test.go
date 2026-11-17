//go:build windows

package cli

import (
	"errors"
	"path/filepath"
	"testing"

	"floe/internal/usererr"
)

// Windows' default places are under the profile (USERPROFILE), or the XDG base directories, as on Linux: HOME and
// LOCALAPPDATA do not stand in for the profile, a relative one is none, and a setting is still an absolute path.
func TestWindowsPlaces(t *testing.T) {
	t.Parallel()

	home, data, config := t.TempDir(), t.TempDir(), t.TempDir()
	for name, c := range map[string]struct {
		env          map[string]string
		store, parts string
	}{
		"the profile": {
			map[string]string{"USERPROFILE": home},
			filepath.Join(home, `.local\share\floe`), filepath.Join(home, `.config\floe\parts`),
		},
		"XDG over the profile": {
			map[string]string{"USERPROFILE": home, "XDG_DATA_HOME": data, "XDG_CONFIG_HOME": config},
			filepath.Join(data, "floe"), filepath.Join(config, `floe\parts`),
		},
		"XDG relative": {
			map[string]string{"USERPROFILE": home, "XDG_DATA_HOME": `d`, "XDG_CONFIG_HOME": `C:c`},
			filepath.Join(home, `.local\share\floe`), filepath.Join(home, `.config\floe\parts`),
		},
		"HOME and LOCALAPPDATA alone": {map[string]string{"HOME": home, "LOCALAPPDATA": home}, "", ""},
		"the profile relative":        {map[string]string{"USERPROFILE": `Users\u`, "HOME": home}, "", ""},
		"the profile of no drive":     {map[string]string{"USERPROFILE": `\Users\u`, "LOCALAPPDATA": home}, "", ""},
		"the profile, HOME elsewhere": {map[string]string{"USERPROFILE": home, "HOME": data}, filepath.Join(home, `.local\share\floe`), filepath.Join(home, `.config\floe\parts`)},
	} {
		a := with(c.env)
		p, err := a.storeDir()
		switch {
		case c.store == "" && !isKey(err, "store_no_dir"):
			t.Errorf("%s: storeDir = %+v, %v, want store_no_dir", name, p, err)
		case c.store != "" && (err != nil || p.dir != c.store):
			t.Errorf("%s: storeDir = %+v, %v, want %s", name, p, err, c.store)
		}
		if p, err := a.ownPlace("parts"); err != nil || p.dir != c.parts {
			t.Errorf("%s: ownPlace(parts) = %+v, %v, want %q", name, p, err, c.parts)
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
