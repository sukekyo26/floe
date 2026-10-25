// Package version holds the floe release version.
package version

// Version is bumped in lockstep with the repository-root VERSION file so that
// `go install` builds (which skip the justfile's ldflags) report the release
// too. `just build` overrides it at link time with -X.
var Version = "0.0.0"
