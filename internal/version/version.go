// Package version holds the floe release version.
package version

import "runtime/debug"

// Version is bumped in lockstep with the repository-root VERSION file so that
// `go install` builds (which skip the justfile's ldflags) report the release
// too. `just build` overrides it at link time with -X.
var Version = "0.4.0"

// String is the version floe prints: Version, with the commit go stamped into a build of a checkout after a "+".
// Release builds pass -buildvcs=false, so they print Version alone.
// LIMIT: a build of a source with no .git, or with -buildvcs=false by hand, prints Version alone like a release.
func String() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}
	return describe(Version, info.Settings)
}

// describe adds to v the first 12 hex digits of vcs.revision in settings (as long as Go's pseudo-versions), and
// ".dirty" when the tree had changes, as semver build metadata.
func describe(v string, settings []debug.BuildSetting) string {
	var rev string
	var dirty bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return v
	}
	v += "+" + rev[:min(12, len(rev))]
	if dirty {
		v += ".dirty"
	}
	return v
}
