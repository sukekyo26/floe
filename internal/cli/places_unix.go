//go:build unix

package cli

// The places floe keeps its own in by default on Unix: under the XDG base directories, else under the home.
var (
	dataBase   = base{key: "XDG_DATA_HOME", underHome: ".local/share"}
	configBase = base{key: "XDG_CONFIG_HOME", underHome: ".config"}
	storeSub   = "floe"
	iconsSub   = "floe-icons"
)
