package icons

import (
	"embed"
	"io/fs"
	"regexp"
)

//go:embed lucide
var embedded embed.FS

// builtinFS holds the icon sets floe ships: lucide/, its index.json, icons and LICENSE, written by
// go test ./internal/icons -run TestBuiltinLucide -update -lucide-zip <lucide-icons-<version>.zip>. It is embedded;
// a test makes it fail.
var builtinFS fs.FS = embedded

// lucideVersion is the release of Lucide whose icons floe ships.
const lucideVersion = "1.72.0"

// lucide is every icon of a release of Lucide (ISC; MIT for those of Feather), lines in currentColor, each in the
// category Lucide gives it. It is built in: its icons are in floe, and it is never fetched.
var lucide = kit{
	name:        "lucide",
	title:       "Lucide",
	page:        "https://github.com/lucide-icons/lucide/releases/tag/" + lucideVersion,
	host:        "",
	size:        "",
	terms:       false,
	builtin:     true,
	categorized: false,
	packages: []pack{{
		what:   "the lucide-icons zip",
		link:   nil, // never fetched
		iconOf: lucideIconOf,
	}},
	version: nil,
	short: map[string][]string{
		"ai":      {"bot"},
		"browser": {"app-window"},
		"db":      {"database"},
		"email":   {"mail"},
		"mobile":  {"smartphone"},
		"pc":      {"monitor"},
		"people":  {"users"},
		"person":  {"user"},
	},
}

// A name has no slash, so no entry of the zip can write outside the set's directory.
var lucideRe = regexp.MustCompile(`^icons/([a-z0-9-]+)\.svg$`)

// lucideIconOf takes every icon of Lucide's zip, its category written after by the test that writes the set
// (lucideCategories), from Lucide's own metadata.
func lucideIconOf(path string) (packed, bool) {
	m := lucideRe.FindStringSubmatch(path)
	if m == nil {
		return packed{version: "", name: "", category: "", dark: false}, false
	}
	return packed{version: "", name: m[1], category: "", dark: false}, true
}
