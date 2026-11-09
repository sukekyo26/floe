// Package docs holds the guide for agents, which floe guide prints.
package docs

import _ "embed"

// Guide is guide.md, the guide for agents.
//
//go:embed guide.md
var Guide []byte
