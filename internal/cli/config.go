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

// config is what floe reads from the environment: for each setting, its value, the mistake it is written
// with (wrong), or why it has no default (absent), its value then none. A command is told of them as it
// uses the setting (settle).
type config struct {
	root      string
	icons     string // empty when there is nowhere to put it, which the icons commands refuse
	templates string // the templates of one's own; empty when there is nowhere to put them
	parts     string // the parts of one's own; empty when there is nowhere to put them
	themes    string // the themes of one's own; empty when there is nowhere to put them
	home      string
	port      int
	wrong     map[setting]*usererr.Error
	absent    map[setting]*usererr.Error
}

func (c *config) base() string {
	return "http://" + daemon.Addr(c.port)
}

// loadConfig reads the settings from getenv, a relative directory under dir, the working directory ("" when
// it cannot be read), so that a server a command starts or finds, whatever its own, means the ones the
// command does.
func loadConfig(getenv func(string) string, dir string) *config {
	c := &config{
		root: "", icons: "", templates: "", parts: "", themes: "", home: getenv("HOME"), port: defaultPort,
		wrong: map[setting]*usererr.Error{}, absent: map[setting]*usererr.Error{},
	}
	if raw := getenv("FLOE_PORT"); raw != "" {
		// Decimal only: hex (0x10) and exponents (1e3) are typos, not ports.
		port, err := strconv.Atoi(raw)
		if !decimalIntRe.MatchString(raw) || err != nil || port < 1 || port > 65535 {
			c.wrong[portSetting] = usererr.New("cli_invalid_port", raw)
		} else {
			c.port = port
		}
	}
	c.readDirs(getenv, dir)
	return c
}

// readDirs reads floe's directories: each its setting, else a default under the XDG base directory, else
// under the home. With none of them, it has no default, and is none to a command that only reads it.
func (c *config) readDirs(getenv func(string) string, dir string) {
	data := xdgDir(getenv, "XDG_DATA_HOME", c.home, ".local", "share")
	conf := xdgDir(getenv, "XDG_CONFIG_HOME", c.home, ".config")
	for _, d := range []struct {
		s               setting
		key, base, name string
		to              *string
	}{
		{storeSetting, "FLOE_DIR", data, "floe", &c.root},
		// Not under FLOE_DIR: icons is a valid slug, so a page could take its place.
		{iconsSetting, "FLOE_ICONS_DIR", data, "floe-icons", &c.icons},
		{templatesSetting, "FLOE_TEMPLATES_DIR", conf, filepath.Join("floe", "templates"), &c.templates},
		{partsSetting, "FLOE_PARTS_DIR", conf, filepath.Join("floe", "parts"), &c.parts},
		{themesSetting, "FLOE_THEMES_DIR", conf, filepath.Join("floe", "themes"), &c.themes},
	} {
		v, mistake := floeDir(getenv, d.key, c.home, dir)
		switch {
		case mistake != nil:
			c.wrong[d.s] = mistake
		case v != "":
			*d.to = v
		case d.base != "":
			*d.to = filepath.Join(d.base, d.name)
		default:
			c.absent[d.s] = usererr.New("cli_no_default", d.key, d.key)
		}
	}
}

// floeDir reads floe's directory setting key: a ~ or ~/ at its start stands for home, so that one written
// in quotes (a .env file, a service unit) does not name a directory "~", and a relative one is under dir.
// ~name (the home of name, to a shell), a ~ without a home and a relative one without dir are refused.
// LIMIT: ~name is refused, not looked up: os/user without cgo reads /etc/passwd alone on linux, which misses
// the users of NSS (LDAP, SSSD); look it up if one comes to need ~name, and refuse a name it cannot find.
func floeDir(getenv func(string) string, key, home, dir string) (string, *usererr.Error) {
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
