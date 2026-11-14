// Package docs holds the docs floe prints for agents, in docs/agents/ and in English alone: the guide (floe guide) and
// the guides to writing a diagram, a part, a template and a theme (floe diagram|part|template|theme guide), the docs
// themselves rather than copies of them. Each links only its own headings, as an agent cannot open another doc
// (TestGuideDocLinks); every md in docs/agents/ is embedded here (TestAgentDocs).
package docs

import _ "embed"

// Guide is guide.md, the guide for agents.
//
//go:embed agents/guide.md
var Guide []byte

// DiagramGuide is diagrams.md, how to write a diagram, whole.
//
//go:embed agents/diagrams.md
var DiagramGuide []byte

// PartGuide is parts.md, how to make a part, a look or a shared schema of one's own, whole.
//
//go:embed agents/parts.md
var PartGuide []byte

// TemplateGuide is templates.md, how to make a template of one's own, whole.
//
//go:embed agents/templates.md
var TemplateGuide []byte

// ThemeGuide is themes.md, how to make a theme of one's own, whole.
//
//go:embed agents/themes.md
var ThemeGuide []byte

// TaskGuide is tasks.md, how to read the task list, put pages on a task and change a task, whole.
//
//go:embed agents/tasks.md
var TaskGuide []byte
