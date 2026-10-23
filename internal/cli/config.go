package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/usererr"
)

const defaultPort = 7325

var decimalIntRe = regexp.MustCompile(`^[0-9]+$`)

// config is what floe reads from the environment. Every command checks it first, and is told every mistake
// of it at once. A directory is nowhere without its setting, an XDG base directory and a home.
type config struct {
	root      string
	icons     string // empty when there is nowhere to put it, which the icons commands refuse
	templates string // the templates of one's own; empty when there is nowhere to put them
	parts     string // the parts of one's own; empty when there is nowhere to put them
	themes    string // the themes of one's own; empty when there is nowhere to put them
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
	c := &config{root: "", icons: "", templates: "", parts: "", themes: "", home: getenv("HOME"), port: defaultPort}
	var found usererr.List
	if raw := getenv("FLOE_PORT"); raw != "" {
		// Decimal only: hex (0x10) and exponents (1e3) are typos, not ports.
		port, err := strconv.Atoi(raw)
		if !decimalIntRe.MatchString(raw) || err != nil || port < 1 || port > 65535 {
			found = append(found, usererr.New("cli_invalid_port", raw))
		} else {
			c.port = port
		}
	}
	c.readDirs(getenv, dir, &found)
	if err := found.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

// readDirs reads floe's directories: each its setting, else a default under the XDG base directory, else
// under the home. With none of them, the icons and the templates, parts and themes of one's own are none
// (the icons commands refuse that, and floe's templates, parts and themes are the only ones), and the pages
// are refused. Each set that cannot be a directory is a mistake of its setting, added to found; the defaults
// that cannot be are told by what is in their way, once for all it holds up.
func (c *config) readDirs(getenv func(string) string, dir string, found *usererr.List) {
	data := xdgDir(getenv, "XDG_DATA_HOME", c.home, ".local", "share")
	conf := xdgDir(getenv, "XDG_CONFIG_HOME", c.home, ".config")
	var defaults []*heldUp
	for _, d := range []struct {
		key, xdg, base, name string
		to                   *string
	}{
		// First, so that one without a place for the pages is told that before the rest.
		{"FLOE_DIR", "XDG_DATA_HOME", data, "floe", &c.root},
		// Not under FLOE_DIR: icons is a valid slug, so a page could take its place.
		{"FLOE_ICONS_DIR", "XDG_DATA_HOME", data, "floe-icons", &c.icons},
		{"FLOE_TEMPLATES_DIR", "XDG_CONFIG_HOME", conf, filepath.Join("floe", "templates"), &c.templates},
		{"FLOE_PARTS_DIR", "XDG_CONFIG_HOME", conf, filepath.Join("floe", "parts"), &c.parts},
		{"FLOE_THEMES_DIR", "XDG_CONFIG_HOME", conf, filepath.Join("floe", "themes"), &c.themes},
	} {
		v, mistake := floeDir(getenv, d.key, c.home, dir)
		switch {
		case mistake != nil:
			*found = append(*found, mistake)
			continue
		case v != "":
			if at := notDir(v); at == v {
				*found = append(*found, usererr.New("cli_dir_is_file", d.key, v, d.key))
			} else if at != "" {
				*found = append(*found, usererr.New("cli_dir_under_file", d.key, v, at, d.key))
			}
		case d.base != "":
			v = filepath.Join(d.base, d.name)
			defaults = holdUp(defaults, notDir(v), d.key, d.xdg)
		case d.to == &c.root:
			*found = append(*found, usererr.New("cli_no_home"))
			continue
		}
		*d.to = v
	}
	for _, h := range defaults {
		keys, xdgs := strings.Join(h.keys, ", "), strings.Join(h.xdgs, ", ")
		*found = append(*found, usererr.New("cli_dirs_default_not_dir", h.at, keys, xdgs))
	}
}

// heldUp is what at, not a directory, holds up: the settings whose defaults would be under it, and the XDG
// base directories that put them there.
type heldUp struct {
	at         string
	keys, xdgs []string
}

// holdUp adds the default of the setting key, which xdg puts under at, to what at holds up ("" for nothing).
func holdUp(all []*heldUp, at, key, xdg string) []*heldUp {
	if at == "" {
		return all
	}
	i := slices.IndexFunc(all, func(h *heldUp) bool { return h.at == at })
	if i < 0 {
		all, i = append(all, &heldUp{at: at, keys: nil, xdgs: nil}), len(all)
	}
	h := all[i]
	h.keys = append(h.keys, key)
	if !slices.Contains(h.xdgs, xdg) {
		h.xdgs = append(h.xdgs, xdg)
	}
	return all
}

// notDir is where dir cannot be a directory: dir, or the nearest of its parents that is there, when that is
// not a directory (a symlink that leads nowhere is there, and not one); "" when it is one. Whether floe may
// write there is left to the commands that write, so that one that only reads is not stopped, and nothing
// is made.
func notDir(dir string) string {
	for at := dir; ; at = filepath.Dir(at) {
		info, err := os.Stat(at)
		switch {
		case err == nil && info.IsDir():
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
