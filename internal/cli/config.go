package cli

import (
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/sukekyo26/floe/internal/usererr"
)

const defaultPort = 7325

var decimalIntRe = regexp.MustCompile(`^[0-9]+$`)

// config is what floe reads from the environment. Every command checks it first.
type config struct {
	root  string
	icons string // empty when there is no home to put it under, which the icons commands refuse
	home  string
	port  int
}

func (c *config) base() string {
	return "http://127.0.0.1:" + strconv.Itoa(c.port)
}

func loadConfig(getenv func(string) string) (*config, error) {
	c := &config{root: "", icons: "", home: getenv("HOME"), port: defaultPort}
	if raw := getenv("FLOE_PORT"); raw != "" {
		// Decimal only: hex (0x10) and exponents (1e3) are typos, not ports.
		port, err := strconv.Atoi(raw)
		if !decimalIntRe.MatchString(raw) || err != nil || port < 1 || port > 65535 {
			return nil, usererr.New("cli_invalid_port", raw)
		}
		c.port = port
	}
	data := getenv("XDG_DATA_HOME")
	if data == "" && c.home != "" {
		data = filepath.Join(c.home, ".local", "share")
	}
	c.root = getenv("FLOE_DIR")
	if c.root == "" {
		if data == "" {
			return nil, usererr.New("cli_no_home")
		}
		c.root = filepath.Join(data, "floe")
	}
	// Not under FLOE_DIR: icons is a valid slug, so a page could take its place.
	c.icons = getenv("FLOE_ICONS_DIR")
	if c.icons == "" && data != "" {
		c.icons = filepath.Join(data, "floe-icons")
	}
	return c, nil
}
