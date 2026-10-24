package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/dirs"
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
	heldBy    map[setting]string // the file in the way of an absent default
}

// dirKeys are the variables that set floe's directories.
var dirKeys = map[setting]string{
	storeSetting: "FLOE_DIR", iconsSetting: "FLOE_ICONS_DIR", templatesSetting: "FLOE_TEMPLATES_DIR",
	partsSetting: "FLOE_PARTS_DIR", themesSetting: "FLOE_THEMES_DIR",
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
		wrong: map[setting]*usererr.Error{}, absent: map[setting]*usererr.Error{}, heldBy: map[setting]string{},
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
// under the home. A setting that cannot be a directory is written wrong. A default that cannot be one, or
// none at all, is absent, and none to a command that only reads it.
func (c *config) readDirs(getenv func(string) string, dir string) {
	data := xdgDir(getenv, "XDG_DATA_HOME", c.home, ".local", "share")
	conf := xdgDir(getenv, "XDG_CONFIG_HOME", c.home, ".config")
	for _, d := range []struct {
		s               setting
		xdg, base, name string
		to              *string
	}{
		{storeSetting, "XDG_DATA_HOME", data, "floe", &c.root},
		// Not under FLOE_DIR: icons is a valid slug, so a page could take its place.
		{iconsSetting, "XDG_DATA_HOME", data, "floe-icons", &c.icons},
		{templatesSetting, "XDG_CONFIG_HOME", conf, filepath.Join("floe", "templates"), &c.templates},
		{partsSetting, "XDG_CONFIG_HOME", conf, filepath.Join("floe", "parts"), &c.parts},
		{themesSetting, "XDG_CONFIG_HOME", conf, filepath.Join("floe", "themes"), &c.themes},
	} {
		key := dirKeys[d.s]
		v, mistake := floeDir(getenv, key, c.home, dir)
		switch {
		case mistake != nil:
			c.wrong[d.s] = mistake
		case v != "":
			if wrong := setNotDir(key, v); wrong != nil {
				c.wrong[d.s] = wrong
			} else {
				*d.to = v
			}
		case d.base != "":
			v = filepath.Join(d.base, d.name)
			if at := notDir(v); at != "" {
				c.absent[d.s] = usererr.New("cli_default_held_up", at, key, d.xdg)
				c.heldBy[d.s] = at
			} else {
				*d.to = v
			}
		default:
			c.absent[d.s] = usererr.New("cli_no_default", key, key)
		}
	}
}

// setNotDir is the mistake of the setting key, v, when v cannot be a directory; nil when it can.
func setNotDir(key, v string) *usererr.Error {
	switch at := notDir(v); at {
	case "":
		return nil
	case v:
		return usererr.New("cli_dir_is_file", key, v, key)
	default:
		return usererr.New("cli_dir_under_file", key, v, at, key)
	}
}

// notDir is where dir cannot be a directory: dir, or the nearest of its parents that is there, when that is
// not a directory (a symlink that leads nowhere is there, and not one); "" when it is one, or may be. Whether
// floe may write there is left to the commands that write, so that one that only reads is not stopped, and
// nothing is made.
func notDir(dir string) string {
	for at := dir; ; at = filepath.Dir(at) {
		info, err := os.Stat(at)
		switch {
		case err == nil && info.IsDir(), errors.Is(err, fs.ErrPermission):
			return ""
		case err == nil:
			return at
		}
		if _, err := os.Lstat(at); err == nil {
			return at
		}
		if filepath.Dir(at) == at {
			return ""
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
	}
	abs, ok := dirs.Under(dir, v)
	if !ok {
		return "", usererr.New("cli_relative_no_dir", key, v)
	}
	return abs, nil
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
