package dirs_test

import "floe/internal/dirs"

var underCases = []underCase{
	{"relative", `C:\w`, "a/../b.yaml", `C:\w\a/../b.yaml`, nil},
	{"cwd ends in a separator", `C:\w\`, "b.yaml", `C:\w\b.yaml`, nil},
	{"from the root of the drive", `C:\w`, `\b.yaml`, `C:\b.yaml`, nil},
	{"from the root of a share", `\\h\s\w`, `\b.yaml`, `\\h\s\b.yaml`, nil},
	{"drive-relative", `C:\w`, "C:b.yaml", "", dirs.ErrDriveRelative},
	{"drive-relative, no cwd", "", "C:b.yaml", "", dirs.ErrDriveRelative},
	{"absolute", `C:\w`, `D:\x.yaml`, `D:\x.yaml`, nil},
	{"relative, no cwd", "", "a.yaml", "", dirs.ErrNoCwd},
}

var dirCases = []dirCase{
	{`C:\w\a.yaml`, `C:\w`},
	{`C:\a.yaml`, `C:\`},
	{`C:\`, `C:\`},
	{`\\h\s\x.yaml`, `\\h\s\`},
	{`\\?\C:\w\a.yaml`, `\\?\C:\w`},
	{`\\?\C:\a.yaml`, `\\?\C:\`},
	{"link/../a.yaml", "link/.."},
	{"/a.yaml", "/"},
	{"C:a.yaml", "C:."},
}

var joinCases = []joinCase{
	{`C:\`, "a.html", `C:\a.html`},
	{`C:\w/`, "a.html", `C:\w/a.html`},
	{`C:\w`, "a.html", `C:\w\a.html`},
}

var holdsCases = []holdsCase{
	{`\\?\C:\w`, `\\?\C:\w\a.html`, true},
	{`\\?\C:\w`, `\\?\C:\w`, false},
	{`\\?\C:\w`, `\\?\C:\wx\a.html`, false},
	{`\\?\C:\`, `\\?\C:\Users\u`, true},
	{`\\?\C:\`, `\\?\C:\`, false},
	{`\\?\C:\`, `\\?\D:\a.html`, false},
}
