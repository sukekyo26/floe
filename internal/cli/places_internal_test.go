package cli

import (
	"path/filepath"

	"floe/internal/link"
)

// homeVars is the environment of a home at home and no other setting: the system's variable of the home alone
// (HOME, Windows' USERPROFILE).
func homeVars(home string) map[string]string {
	return map[string]string{link.HomeKey: home}
}

// defaultPlace is where floe keeps what (store, icons, parts, themes, templates) by default in the home of homeVars,
// on every system: under ~/.local/share and ~/.config.
func defaultPlace(home, what string) string {
	switch what {
	case "store":
		return filepath.Join(home, ".local", "share", "floe")
	case "icons":
		return filepath.Join(home, ".local", "share", "floe-icons")
	}
	return filepath.Join(home, ".config", "floe", what)
}

// placeVar is the variable a mistake of no default place names: the home's.
func placeVar(string) string { return link.HomeKey }
