package cli

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"HOME": "/h"}
	getenv := func(k string) string { return vars[k] }
	c, err := loadConfig(getenv)
	if err != nil || c.root != "/h/.local/share/floe" || c.port != 7325 || c.idle != 30*time.Minute {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	vars["XDG_DATA_HOME"] = "/x"
	vars["FLOE_IDLE_MINUTES"] = "100000"
	vars["FLOE_PORT"] = "8000"
	if c, err := loadConfig(getenv); err != nil || c.root != "/x/floe" || c.port != 8000 ||
		c.idle != (1<<31-1)*time.Millisecond || c.base() != "http://127.0.0.1:8000" {
		t.Errorf("XDG_DATA_HOME, a capped idle time and a port = %+v, %v", c, err)
	}
	vars["FLOE_DIR"] = "/d"
	vars["FLOE_IDLE_MINUTES"] = ".5"
	if c, err := loadConfig(getenv); err != nil || c.root != "/d" || c.idle != 30*time.Second {
		t.Errorf("FLOE_DIR and half a minute = %+v, %v", c, err)
	}
}
