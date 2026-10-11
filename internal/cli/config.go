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
	root string
	home string
	port int
}

func (c *config) base() string {
	return "http://127.0.0.1:" + strconv.Itoa(c.port)
}

func loadConfig(getenv func(string) string) (*config, error) {
	c := &config{root: "", home: getenv("HOME"), port: defaultPort}
	if raw := getenv("FLOE_PORT"); raw != "" {
		// Decimal only: hex (0x10) and exponents (1e3) are typos, not ports.
		port, err := strconv.Atoi(raw)
		if !decimalIntRe.MatchString(raw) || err != nil || port < 1 || port > 65535 {
			return nil, usererr.New("cli_invalid_port", raw)
		}
		c.port = port
	}
	c.root = getenv("FLOE_DIR")
	if c.root == "" {
		data := getenv("XDG_DATA_HOME")
		if data == "" {
			if c.home == "" {
				return nil, usererr.New("cli_no_home")
			}
			data = filepath.Join(c.home, ".local", "share")
		}
		c.root = filepath.Join(data, "floe")
	}
	return c, nil
}
