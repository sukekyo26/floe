package cli

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Every setting that is not right, the port and the directories, is told at once, without floe's place in a file: they are
// apart from each other.
func TestLoadConfigMistakes(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"HOME": "/h", "FLOE_PORT": "abc", "FLOE_PARTS_DIR": "~bob/p", "FLOE_THEMES_DIR": "~bob/t"}
	_, err := loadConfig(func(k string) string { return vars[k] }, "/w")
	shown := usererr.Show(i18n.EnglishCatalog(), err)
	if lines := strings.Split(shown.Named, "\n"); len(lines) != 3 || !strings.HasPrefix(lines[0], "floe: invalid FLOE_PORT") ||
		!strings.HasPrefix(lines[1], "floe: FLOE_PARTS_DIR is ~bob/p") || !strings.HasPrefix(lines[2], "floe: FLOE_THEMES_DIR is ~bob/t") || shown.Placed {
		t.Errorf("two directories not right = %+v", shown)
	}
}

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
