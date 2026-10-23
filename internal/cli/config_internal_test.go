package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"HOME": "/h"}
	getenv := func(k string) string { return vars[k] }
	c, err := loadConfig(getenv, "/w")
	if err != nil || c.root != "/h/.local/share/floe" || c.icons != "/h/.local/share/floe-icons" || c.port != 7325 ||
		c.templates != "/h/.config/floe/templates" || c.parts != "/h/.config/floe/parts" || c.themes != "/h/.config/floe/themes" {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	vars["XDG_DATA_HOME"] = "/x"
	vars["FLOE_PORT"] = "8000"
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/x/floe" || c.icons != "/x/floe-icons" || c.port != 8000 || c.base() != "http://127.0.0.1:8000" {
		t.Errorf("XDG_DATA_HOME and a port = %+v, %v", c, err)
	}
	vars["XDG_CONFIG_HOME"] = "/c"
	if c, err := loadConfig(getenv, "/w"); err != nil || c.templates != "/c/floe/templates" || c.parts != "/c/floe/parts" ||
		c.themes != "/c/floe/themes" {
		t.Errorf("XDG_CONFIG_HOME = %+v, %v", c, err)
	}
	vars["FLOE_DIR"] = "/d"
	vars["FLOE_ICONS_DIR"] = "/i"
	vars["FLOE_TEMPLATES_DIR"] = "/t"
	vars["FLOE_PARTS_DIR"] = "/p"
	vars["FLOE_THEMES_DIR"] = "/th"
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/d" || c.icons != "/i" || c.templates != "/t" || c.parts != "/p" ||
		c.themes != "/th" {
		t.Errorf("FLOE_DIR, FLOE_ICONS_DIR, FLOE_TEMPLATES_DIR, FLOE_PARTS_DIR and FLOE_THEMES_DIR = %+v, %v", c, err)
	}
	// Without a home, FLOE_DIR is enough for pages; the icons are refused when used, and the built-in
	// templates are the only ones.
	vars = map[string]string{"FLOE_DIR": "/d"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/d" || c.icons != "" || c.templates != "" || c.parts != "" ||
		c.themes != "" {
		t.Errorf("FLOE_DIR without a home = %+v, %v", c, err)
	}
	// A relative directory is under the working directory; a relative XDG directory is none, as its spec
	// says.
	vars = map[string]string{"HOME": "/h", "FLOE_DIR": "pages", "FLOE_ICONS_DIR": "icons", "FLOE_TEMPLATES_DIR": "t/"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/w/pages" || c.icons != "/w/icons" || c.templates != "/w/t" {
		t.Errorf("relative directories = %+v, %v", c, err)
	}
	if _, err := loadConfig(getenv, ""); err == nil || !strings.Contains(err.Error(), "FLOE_DIR, the relative directory pages") {
		t.Errorf("a relative directory without a working directory = %v", err)
	}
	vars = map[string]string{"HOME": "/h", "XDG_DATA_HOME": "data", "XDG_CONFIG_HOME": "conf"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/h/.local/share/floe" || c.icons != "/h/.local/share/floe-icons" ||
		c.templates != "/h/.config/floe/templates" {
		t.Errorf("relative XDG directories = %+v, %v", c, err)
	}
	// A ~ or ~/ written in quotes stands for the home; ~name, a home to a shell, is refused.
	vars = map[string]string{"HOME": "/h", "FLOE_DIR": "~/pages", "FLOE_TEMPLATES_DIR": "~", "FLOE_ICONS_DIR": "~/icons"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/h/pages" || c.templates != "/h" || c.icons != "/h/icons" {
		t.Errorf("directories with ~ = %+v, %v", c, err)
	}
	vars = map[string]string{"HOME": "/h", "XDG_DATA_HOME": "~/data", "XDG_CONFIG_HOME": "~/cfg"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/h/data/floe" || c.templates != "/h/cfg/floe/templates" {
		t.Errorf("XDG directories with ~ = %+v, %v", c, err)
	}
	vars = map[string]string{"HOME": "/h", "FLOE_ICONS_DIR": "~x"}
	if _, err := loadConfig(getenv, "/w"); err == nil || !strings.Contains(err.Error(), "FLOE_ICONS_DIR is ~x: floe takes only ~ and ~/") {
		t.Errorf("~name = %v", err)
	}
	// Without a home, a floe directory's ~ is refused, and an XDG one's ignored as a relative one is.
	vars = map[string]string{"FLOE_DIR": "~/pages"}
	if _, err := loadConfig(getenv, "/w"); err == nil || !strings.Contains(err.Error(), "FLOE_DIR is ~/pages, whose ~ stands for a home") {
		t.Errorf("~ without a home = %v", err)
	}
	vars = map[string]string{"FLOE_DIR": "/d", "XDG_DATA_HOME": "~/data"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/d" || c.icons != "" {
		t.Errorf("an XDG directory with ~ without a home = %+v, %v", c, err)
	}
	// Without a place for the pages, that is told first.
	vars = map[string]string{"FLOE_ICONS_DIR": "~/icons"}
	if _, err := loadConfig(getenv, "/w"); err == nil || !strings.Contains(err.Error(), "HOME is not set; set HOME or FLOE_DIR") {
		t.Errorf("no place for the pages and an icons directory of ~ = %v", err)
	}
	// An XDG directory is not floe's: one of ~name is ignored, as a relative one is.
	vars = map[string]string{"HOME": "/h", "XDG_CONFIG_HOME": "~foo"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.templates != "/h/.config/floe/templates" {
		t.Errorf("an XDG directory of ~name = %+v, %v", c, err)
	}
	// An absolute directory is cleaned, as a relative one is joined.
	vars = map[string]string{"FLOE_DIR": "/d/"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/d" {
		t.Errorf("an absolute directory with a slash at the end = %+v, %v", c, err)
	}
}

// A directory setting that is a file, or under one, is told by its name, every one at once with the other
// mistakes of the settings; one not there yet, under a directory, is taken and not made.
func TestLoadConfigDirs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	file := filepath.Join(tmp, "afile")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(tmp, filepath.Join(tmp, "link")); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(tmp, "dangling")
	if err := os.Symlink(filepath.Join(tmp, "nowhere"), dangling); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"FLOE_DIR", "FLOE_ICONS_DIR", "FLOE_TEMPLATES_DIR", "FLOE_PARTS_DIR", "FLOE_THEMES_DIR"} {
		vars := map[string]string{"HOME": tmp, key: filepath.Join(file, "x")}
		_, err := loadConfig(func(k string) string { return vars[k] }, tmp)
		if want := key + ", " + file + "/x, is under " + file + ", which is not a directory; set " + key + " to a directory"; err == nil || err.Error() != want {
			t.Errorf("%s under a file = %v, want %q", key, err, want)
		}
	}
	vars := map[string]string{"HOME": tmp, "FLOE_PORT": "0", "FLOE_PARTS_DIR": file, "FLOE_THEMES_DIR": filepath.Join(file, "x")}
	_, err := loadConfig(func(k string) string { return vars[k] }, tmp)
	if err == nil || err.Error() != `invalid FLOE_PORT "0": use a port number from 1 to 65535`+"\n"+
		"FLOE_PARTS_DIR, "+file+", is not a directory; set FLOE_PARTS_DIR to a directory\n"+
		"FLOE_THEMES_DIR, "+file+"/x, is under "+file+", which is not a directory; set FLOE_THEMES_DIR to a directory" {
		t.Errorf("several mistakes of the settings = %v", err)
	}
	// A symlink that leads nowhere cannot be made a directory, nor can what is under it.
	vars = map[string]string{"HOME": tmp, "FLOE_DIR": dangling, "FLOE_THEMES_DIR": filepath.Join(dangling, "x")}
	_, err = loadConfig(func(k string) string { return vars[k] }, tmp)
	if err == nil || err.Error() != "FLOE_DIR, "+dangling+", is not a directory; set FLOE_DIR to a directory\n"+
		"FLOE_THEMES_DIR, "+dangling+"/x, is under "+dangling+", which is not a directory; set FLOE_THEMES_DIR to a directory" {
		t.Errorf("a symlink that leads nowhere = %v", err)
	}
	// A default in the way of a file is told once by that file, with the settings it holds up and what puts
	// them there; after the mistakes of those set.
	home := filepath.Join(tmp, "h")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	conf := home + "/.config is not a directory, and FLOE_TEMPLATES_DIR, FLOE_PARTS_DIR, FLOE_THEMES_DIR would be under it by default; " +
		"make it a directory, or set XDG_CONFIG_HOME or each of them elsewhere"
	for _, c := range []struct {
		what string
		vars map[string]string
		want string
	}{
		{"a file in the way of the defaults", map[string]string{"HOME": home}, conf},
		{
			"XDG_CONFIG_HOME a file",
			map[string]string{"HOME": home, "XDG_CONFIG_HOME": file},
			file + " is not a directory, and FLOE_TEMPLATES_DIR, FLOE_PARTS_DIR, FLOE_THEMES_DIR would be under it by default; " +
				"make it a directory, or set XDG_CONFIG_HOME or each of them elsewhere",
		},
		{
			"HOME a file",
			map[string]string{"HOME": file},
			file + " is not a directory, and FLOE_DIR, FLOE_ICONS_DIR, FLOE_TEMPLATES_DIR, FLOE_PARTS_DIR, FLOE_THEMES_DIR would be under it " +
				"by default; make it a directory, or set XDG_DATA_HOME, XDG_CONFIG_HOME or each of them elsewhere",
		},
		{
			"one of the defaults set",
			map[string]string{"HOME": home, "FLOE_TEMPLATES_DIR": tmp},
			home + "/.config is not a directory, and FLOE_PARTS_DIR, FLOE_THEMES_DIR would be under it by default; " +
				"make it a directory, or set XDG_CONFIG_HOME or each of them elsewhere",
		},
		{
			"a setting before the defaults",
			map[string]string{"HOME": home, "FLOE_DIR": file},
			"FLOE_DIR, " + file + ", is not a directory; set FLOE_DIR to a directory\n" + conf,
		},
	} {
		if _, err := loadConfig(func(k string) string { return c.vars[k] }, tmp); err == nil || err.Error() != c.want {
			t.Errorf("%s = %v, want %q", c.what, err, c.want)
		}
	}
	vars = map[string]string{"HOME": tmp, "FLOE_DIR": filepath.Join(tmp, "new", "pages"), "FLOE_PARTS_DIR": filepath.Join(tmp, "link")}
	if c, err := loadConfig(func(k string) string { return vars[k] }, tmp); err != nil || c.root != filepath.Join(tmp, "new", "pages") {
		t.Errorf("a directory not there yet and a symlink to a directory = %+v, %v", c, err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "new")); !os.IsNotExist(err) {
		t.Errorf("reading the settings made a directory: %v", err)
	}
}
