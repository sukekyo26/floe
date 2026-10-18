package cli

import (
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"HOME": "/h"}
	getenv := func(k string) string { return vars[k] }
	c, err := loadConfig(getenv, "/w")
	if err != nil || c.root != "/h/.local/share/floe" || c.icons != "/h/.local/share/floe-icons" || c.port != 7325 ||
		c.templates != "/h/.config/floe/templates" {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	vars["XDG_DATA_HOME"] = "/x"
	vars["FLOE_PORT"] = "8000"
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/x/floe" || c.icons != "/x/floe-icons" || c.port != 8000 || c.base() != "http://127.0.0.1:8000" {
		t.Errorf("XDG_DATA_HOME and a port = %+v, %v", c, err)
	}
	vars["XDG_CONFIG_HOME"] = "/c"
	if c, err := loadConfig(getenv, "/w"); err != nil || c.templates != "/c/floe/templates" {
		t.Errorf("XDG_CONFIG_HOME = %+v, %v", c, err)
	}
	vars["FLOE_DIR"] = "/d"
	vars["FLOE_ICONS_DIR"] = "/i"
	vars["FLOE_TEMPLATES_DIR"] = "/t"
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/d" || c.icons != "/i" || c.templates != "/t" {
		t.Errorf("FLOE_DIR, FLOE_ICONS_DIR and FLOE_TEMPLATES_DIR = %+v, %v", c, err)
	}
	// Without a home, FLOE_DIR is enough for pages; the icons are refused when used, and the built-in
	// templates are the only ones.
	vars = map[string]string{"FLOE_DIR": "/d"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/d" || c.icons != "" || c.templates != "" {
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
	// A ~ written in quotes stands for the home, as a shell would have expanded it; ~name is a name.
	vars = map[string]string{"HOME": "/h", "FLOE_DIR": "~/pages", "FLOE_TEMPLATES_DIR": "~", "XDG_DATA_HOME": "~/data", "FLOE_ICONS_DIR": "~x"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/h/pages" || c.templates != "/h" || c.icons != "/w/~x" {
		t.Errorf("directories with ~ = %+v, %v", c, err)
	}
	vars = map[string]string{"HOME": "/h", "XDG_DATA_HOME": "~/data"}
	if c, err := loadConfig(getenv, "/w"); err != nil || c.root != "/h/data/floe" {
		t.Errorf("XDG_DATA_HOME with ~ = %+v, %v", c, err)
	}
	vars = map[string]string{"FLOE_DIR": "~/pages"}
	if _, err := loadConfig(getenv, "/w"); err == nil || !strings.Contains(err.Error(), "FLOE_DIR is ~/pages, whose ~ stands for a home") {
		t.Errorf("~ without a home = %v", err)
	}
}
