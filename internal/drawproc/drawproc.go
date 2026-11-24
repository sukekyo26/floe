// Package drawproc draws pages for the server in a process of its own, floe __draw, which watches its memory
// (guard) and which the server ends past a time: a page whose expressions or page.html take gigabytes, or that
// never ends, stops that process, not the server. The server never draws (tmpl) itself: floe __draw draws with
// drawhere.
package drawproc

import (
	"context"
	"errors"
	"time"

	"floe/internal/guard"
	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

// Job is a page to draw: tmpl.Draw's data file, its options, and where its images are read from.
type Job struct {
	File     string `json:"file"`
	Source   []byte `json:"source"`
	Theme    string `json:"theme,omitzero"`
	Default  string `json:"default,omitzero"`
	Untitled string `json:"untitled,omitzero"`
	CutOff   bool   `json:"cutOff,omitzero"`
	// The images: the directory ImagesRoot (files.NewRoot), else those sent (a publish's), else none.
	ImagesRoot string            `json:"imagesRoot,omitzero"`
	Sent       map[string][]byte `json:"sent,omitzero"`
	// ImagesDir is tmpl.Options.ImagesDir.
	ImagesDir string `json:"imagesDir,omitzero"`
}

// Result is what drawing a Job gave: tmpl.Draw's page, warnings and mistakes (or floe's failure), and, of the images
// sent, those it read, in the order first read, with the places of the data file that name each.
type Result struct {
	Page     []byte
	Warnings []*usererr.Error
	Err      error
	Read     []string
	Places   map[string][]yamlpos.Pos
}

// Drawer draws pages, a Session at a time.
type Drawer interface {
	// Start begins a session of drawings under ctx, which ends them when it ends. Its caller closes it.
	Start(ctx context.Context) Session
}

// Session draws pages one after another, until it is closed.
type Session interface {
	Draw(job Job) Result
	Close()
}

// Draw draws one job with d under ctx.
func Draw(ctx context.Context, d Drawer, job Job) Result {
	s := d.Start(ctx)
	defer s.Close()
	return s.Draw(job)
}

// Limit is how long the server lets one page be drawn.
const Limit = 30 * time.Second

// memoryLimit is what floe __draw may hold, as its watch tells it.
const memoryLimit = guard.Limit

// failed is a Result of floe's failure err.
func failed(err error) Result {
	return Result{Page: nil, Warnings: nil, Err: err, Read: nil, Places: nil}
}

// errEnded is the failure of a session whose context ended.
var errEnded = errors.New("the drawing was called off")
