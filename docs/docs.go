// Package docs holds the texts floe prints for agents: the guide (floe guide) and the guide to writing a
// diagram (floe diagram guide), the docs themselves rather than copies of them.
package docs

import _ "embed"

// Guide is guide.md, the guide for agents.
//
//go:embed guide.md
var Guide []byte

// DiagramGuide is diagrams.md, how to write a diagram, whole: it holds no link to another doc, which an agent
// cannot open (TestDiagramGuideLinks).
//
//go:embed diagrams.md
var DiagramGuide []byte
