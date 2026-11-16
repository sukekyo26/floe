//go:build unix

package dirs_test

import "floe/internal/dirs"

var underCases = []underCase{
	{"relative", "/w", "a/../b.yaml", "/w/a/../b.yaml", nil},
	{"cwd ends in /", "/w/", "b.yaml", "/w/b.yaml", nil},
	{"absolute", "/w", "/x//y.yaml", "/x//y.yaml", nil},
	{"relative, no cwd", "", "a.yaml", "", dirs.ErrNoCwd},
	{"absolute, no cwd", "", "/a.yaml", "/a.yaml", nil},
	{"a name with a colon", "/w", "C:b.yaml", "/w/C:b.yaml", nil},
}

var dirCases = []dirCase{
	{"link/../a.yaml", "link/.."},
	{"/a.yaml", "/"},
	{"/w/a.yaml", "/w"},
	{"/", "/"},
	{`a\b.yaml`, "."},
}

var joinCases = []joinCase{
	{"link/..", "a.html", "link/../a.html"},
	{"/", "a.html", "/a.html"},
	{"a//", "b", "a//b"},
}
