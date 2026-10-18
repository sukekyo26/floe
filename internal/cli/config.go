package cli

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/usererr"
)

const defaultPort = 7325

var decimalIntRe = regexp.MustCompile(`^[0-9]+$`)

// config is what floe reads from the environment. Every command checks it first. A directory is nowhere
// without its setting, an XDG base directory and a home.
type config struct {
	root      string
	icons     string // empty when there is nowhere to put it, which the icons commands refuse
	templates string // the templates of one's own; empty when there is nowhere to put them
	home      string
	port      int
}

func (c *config) base() string {
	return "http://" + daemon.Addr(c.port)
}

// loadConfig reads the settings from getenv, a relative directory under dir, the working directory ("" when
// it cannot be read), so that a server a command starts or finds, whatever its own, means the ones the
// command does.
func loadConfig(getenv func(string) string, dir string) (*config, error) {
	c := &config{root: "", icons: "", templates: "", home: getenv("HOME"), port: defaultPort}
	if raw := getenv("FLOE_PORT"); raw != "" {
		// Decimal only: hex (0x10) and exponents (1e3) are typos, not ports.
		port, err := strconv.Atoi(raw)
		if !decimalIntRe.MatchString(raw) || err != nil || port < 1 || port > 65535 {
			return nil, usererr.New("cli_invalid_port", raw)
		}
		c.port = port
	}
	if err := c.readDirs(getenv, dir); err != nil {
		return nil, err
	}
	return c, nil
}

// readDirs reads floe's directories: each its setting, else a default under the XDG base directory, else
// under the home. With none of them, the icons and the templates of one's own are none (the icons
// commands refuse that, and the built-in templates are the only ones), and the pages are refused.
func (c *config) readDirs(getenv func(string) string, dir string) error {
	data := xdgDir(getenv, "XDG_DATA_HOME", c.home, ".local", "share")
	conf := xdgDir(getenv, "XDG_CONFIG_HOME", c.home, ".config")
	for _, d := range []struct {
		key, base, name string
		to              *string
	}{
		// First, so that one without a place for the pages is told that before the rest.
		{"FLOE_DIR", data, "floe", &c.root},
		// Not under FLOE_DIR: icons is a valid slug, so a page could take its place.
		{"FLOE_ICONS_DIR", data, "floe-icons", &c.icons},
		{"FLOE_TEMPLATES_DIR", conf, filepath.Join("floe", "templates"), &c.templates},
	} {
		v, err := floeDir(getenv, d.key, c.home, dir)
		if err != nil {
			return err
		}
		if v == "" && d.base != "" {
			v = filepath.Join(d.base, d.name)
		}
		if v == "" && d.to == &c.root {
			return usererr.New("cli_no_home")
		}
		*d.to = v
	}
	return nil
}

// floeDir reads floe's directory setting key: a ~ or ~/ at its start stands for home, so that one written
// in quotes (a .env file, a service unit) does not name a directory "~", and a relative one is under dir.
// ~name (the home of name, to a shell), a ~ without a home and a relative one without dir are refused.
// LIMIT: ~name is refused, not looked up: os/user without cgo reads /etc/passwd alone on linux, which misses
// the users of NSS (LDAP, SSSD); look it up if one comes to need ~name, and refuse a name it cannot find.
func floeDir(getenv func(string) string, key, home, dir string) (string, error) {
	v := getenv(key)
	switch {
	case v == "":
		return "", nil
	case ofHome(v) && home == "":
		return "", usererr.New("cli_tilde_no_home", key, v, key)
	case ofHome(v):
		return filepath.Join(home, v[1:]), nil
	case strings.HasPrefix(v, "~"):
		return "", usererr.New("cli_tilde_name", key, v, v)
	case filepath.IsAbs(v):
		return filepath.Clean(v), nil
	case dir == "":
		return "", usererr.New("cli_relative_no_dir", key, v)
	}
	return filepath.Join(dir, v), nil
}

// xdgDir reads the XDG base directory key, a ~ or ~/ at its start standing for home, else the default
// under home: "" for none. One that is still relative is none, as its spec calls it invalid.
func xdgDir(getenv func(string) string, key, home string, under ...string) string {
	v := getenv(key)
	if ofHome(v) && home != "" {
		v = filepath.Join(home, v[1:])
	}
	switch {
	case filepath.IsAbs(v):
		return filepath.Clean(v)
	case home != "":
		return filepath.Join(append([]string{home}, under...)...)
	}
	return ""
}

// ofHome reports whether v starts with what a shell takes for one's home: ~ or ~/.
func ofHome(v string) bool {
	return v == "~" || strings.HasPrefix(v, "~/")
}
