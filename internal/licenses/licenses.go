// Package licenses holds the license notices floe licenses prints: floe's LICENSE, then the license files of the Go
// standard library, of every module built into a release binary and of the Lucide icons, as each wrote them.
package licenses

import _ "embed"

// Notices is NOTICES.txt, written by just licenses from go list -deps of every release target; TestNotices fails
// when it is not what that gives.
//
// LIMIT: only files named as licenses are copied. Files a module embeds under its own license (jsonschema's
// metaschemas of json-schema.org) and the tables Go and golang.org/x/text make from Unicode's data are taken to be
// under the notices of the module that ships them, as it ships them; add a notice by hand if one says otherwise.
//
//go:embed NOTICES.txt
var Notices []byte
