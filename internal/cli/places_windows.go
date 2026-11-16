package cli

import "path/filepath"

// The places floe keeps its own in by default on Windows: all under %LOCALAPPDATA%\floe, which folder redirection
// never moves onto a share (%APPDATA% it may, which floe refuses); the store a level down, so that its pages'
// directories do not meet the parts' and the others'.
var (
	dataBase   = base{key: "LOCALAPPDATA", underHome: ""}
	configBase = dataBase
	storeSub   = filepath.Join("floe", "pages")
	iconsSub   = filepath.Join("floe", "icons")
)
