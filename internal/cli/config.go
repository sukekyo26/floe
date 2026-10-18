package cli

import (
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/usererr"
)

const defaultPort = 7325

var decimalIntRe = regexp.MustCompile(`^[0-9]+$`)

// config is what floe reads from the environment. Every command checks it first.
type config struct {
	root      string
	icons     string // empty when there is no home to put it under, which the icons commands refuse
	templates string // the templates of one's own; empty when there is no home to put them under
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
	data := xdg(getenv, "XDG_DATA_HOME")
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
	c.templates = templatesDir(getenv, c.home)
	if err := c.resolve(dir); err != nil {
		return nil, err
	}
	return c, nil
}

// resolve puts the relative directories of c under dir, refusing one when dir is "".
func (c *config) resolve(dir string) error {
	for _, d := range []struct {
		name string
		path *string
	}{{"FLOE_DIR", &c.root}, {"FLOE_ICONS_DIR", &c.icons}, {"FLOE_TEMPLATES_DIR", &c.templates}} {
		if dir == "" && *d.path != "" && !filepath.IsAbs(*d.path) {
			return usererr.New("cli_relative_no_dir", d.name, *d.path)
		}
		*d.path = under(dir, *d.path)
	}
	return nil
}

// xdg reads the XDG base directory key, "" for none or a relative one, which the XDG spec calls invalid.
func xdg(getenv func(string) string, key string) string {
	if dir := getenv(key); filepath.IsAbs(dir) {
		return dir
	}
	return ""
}

// templatesDir is where the templates of one's own are, or "" when there is no home to put them under.
func templatesDir(getenv func(string) string, home string) string {
	if dir := getenv("FLOE_TEMPLATES_DIR"); dir != "" {
		return dir
	}
	conf := xdg(getenv, "XDG_CONFIG_HOME")
	if conf == "" && home != "" {
		conf = filepath.Join(home, ".config")
	}
	if conf == "" {
		return ""
	}
	return filepath.Join(conf, "floe", "templates")
}
