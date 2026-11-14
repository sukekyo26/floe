package icons

import (
	"embed"
	"io/fs"
	"regexp"
)

// builtinFS holds the icon sets floe ships: lucide/, its index.json, icons and LICENSE, written by
// go test ./internal/icons -run TestBuiltinLucide -update -lucide-zip <lucide-icons-<version>.zip>.
//
//go:embed lucide
var embedded embed.FS

// builtinFS is embedded; a test makes it fail.
var builtinFS fs.FS = embedded

// lucideVersion is the release of Lucide whose icons floe ships.
const lucideVersion = "1.72.0"

// lucide is a hundred icons of Lucide (ISC; MIT for those of Feather), lines in currentColor, of the things a
// diagram of any kind shows: people, places, devices, networks, data, files, security, messages, work, status
// and money. It is built in: its icons are in floe, and it is never fetched.
var lucide = kit{
	name:    "lucide",
	title:   "Lucide",
	page:    "https://github.com/lucide-icons/lucide/releases/tag/" + lucideVersion,
	host:    "",
	size:    "",
	terms:   false,
	builtin: true,
	packages: []pack{{
		what:   "the lucide-icons zip",
		link:   regexp.MustCompile(`^$`),
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

// lucidePick is the icons floe ships, by name, with the category a search prints.
// LIMIT: names and categories are kept by hand. A release that renames an icon fails TestBuiltinLucide on its
// -update; mend the name then. Take them from Lucide's categories if keeping them comes to cost.
var lucidePick = map[string]string{
	"user": "people", "users": "people", "bot": "people",
	"building": "places", "building-complex": "places", "store": "places", "house": "places",
	"factory": "places", "warehouse": "places", "map-pin": "places", "truck": "places",
	"monitor": "devices", "laptop": "devices", "smartphone": "devices", "tablet": "devices",
	"server": "devices", "hard-drive": "devices", "cpu": "devices", "router": "devices",
	"globe": "network", "cloud": "network", "wifi": "network", "network": "network",
	"radio-tower": "network", "cable": "network", "plug": "network", "link": "network",
	"waypoints": "network", "workflow": "network", "webhook": "network", "app-window": "network",
	"code": "development", "terminal": "development", "layout-dashboard": "development",
	"git-branch": "development", "git-merge": "development", "bug": "development",
	"rocket": "development", "container": "development",
	"database": "data", "archive": "data", "box": "data", "package": "data", "layers": "data",
	"table": "data", "chart-column": "data", "chart-line": "data", "chart-pie": "data",
	"search": "data", "funnel": "data",
	"file": "files", "file-text": "files", "file-code": "files", "file-braces": "files",
	"files": "files", "folder": "files", "image": "files",
	"key": "security", "key-round": "security", "lock": "security", "lock-open": "security",
	"shield": "security", "shield-check": "security", "fingerprint-pattern": "security",
	"id-card": "security", "badge-check": "security",
	"mail": "messages", "inbox": "messages", "send": "messages", "message-square": "messages",
	"bell": "messages", "phone": "messages", "calendar": "messages",
	"settings": "work", "wrench": "work", "zap": "work", "brain": "work", "sparkles": "work",
	"gauge": "work", "activity": "work", "clock": "work", "timer": "work", "hourglass": "work",
	"refresh-cw": "work", "list-checks": "work", "play": "work", "clipboard-list": "work",
	"book-open": "work", "trash": "work", "download": "work", "upload": "work",
	"circle-check": "status", "circle-x": "status", "triangle-exclamation-point": "status",
	"info": "status", "flag": "status",
	"credit-card": "money", "shopping-cart": "money", "wallet": "money", "receipt": "money",
}

// A name has no slash, so no entry of the zip can write outside the set's directory.
var lucideRe = regexp.MustCompile(`^icons/([a-z0-9-]+)\.svg$`)

func lucideIconOf(path string) (packed, bool) {
	m := lucideRe.FindStringSubmatch(path)
	if m == nil {
		return packed{version: "", name: "", category: "", dark: false}, false
	}
	category, ok := lucidePick[m[1]]
	if !ok {
		return packed{version: "", name: "", category: "", dark: false}, false
	}
	return packed{version: "", name: m[1], category: category, dark: false}, true
}
