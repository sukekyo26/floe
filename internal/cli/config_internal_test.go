package cli

import (
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"HOME": "/h"}
	getenv := func(k string) string { return vars[k] }
	c, err := loadConfig(getenv)
	if err != nil || c.root != "/h/.local/share/floe" || c.icons != "/h/.local/share/floe-icons" || c.port != 7325 {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	vars["XDG_DATA_HOME"] = "/x"
	vars["FLOE_PORT"] = "8000"
	if c, err := loadConfig(getenv); err != nil || c.root != "/x/floe" || c.icons != "/x/floe-icons" || c.port != 8000 || c.base() != "http://127.0.0.1:8000" {
		t.Errorf("XDG_DATA_HOME and a port = %+v, %v", c, err)
	}
	vars["FLOE_DIR"] = "/d"
	vars["FLOE_ICONS_DIR"] = "/i"
	if c, err := loadConfig(getenv); err != nil || c.root != "/d" || c.icons != "/i" {
		t.Errorf("FLOE_DIR and FLOE_ICONS_DIR = %+v, %v", c, err)
	}
	// Without a home, FLOE_DIR is enough for pages; the icons are refused when used.
	vars = map[string]string{"FLOE_DIR": "/d"}
	if c, err := loadConfig(getenv); err != nil || c.root != "/d" || c.icons != "" {
		t.Errorf("FLOE_DIR without a home = %+v, %v", c, err)
	}
}
