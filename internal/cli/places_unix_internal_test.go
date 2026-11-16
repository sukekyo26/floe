//go:build unix

package cli

import "path/filepath"

// homeVars is the environment of a home at home and no other setting: on Unix, HOME alone.
func homeVars(home string) map[string]string {
	return map[string]string{"HOME": home}
}

// defaultPlace is where floe keeps what (store, icons, parts, themes, templates) by default in the home of homeVars,
// as Unix has it: under ~/.local/share and ~/.config.
func defaultPlace(home, what string) string {
	switch what {
	case "store":
		return filepath.Join(home, ".local", "share", "floe")
	case "icons":
		return filepath.Join(home, ".local", "share", "floe-icons")
	}
	return filepath.Join(home, ".config", "floe", what)
}

// placeVar is the variable a mistake of no default place of what names (store, icons, parts, …, skills).
func placeVar(string) string { return "HOME" }
