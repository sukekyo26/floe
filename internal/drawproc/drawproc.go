// Package drawproc draws pages for the server in a process of its own, floe __draw, which watches its memory
// (guard) and which the server ends past a time: a page whose expressions or page.html take gigabytes, or that
// never ends, stops that process, not the server. The server never draws (tmpl) itself.
package drawproc

import (
	"context"
	"errors"
	"io/fs"
	"testing/fstest"
	"time"

	"floe/internal/diagram"
	"floe/internal/files"
	"floe/internal/guard"
	"floe/internal/kit"
	"floe/internal/tmpl"
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

// One draws job with k and the icons in the process it runs in: what floe __draw does for each job, and Here.
func One(k *kit.Kit, icons diagram.Icons, job Job) Result {
	o := tmpl.Options{
		Theme: job.Theme, Default: job.Default, Untitled: job.Untitled, Images: nil, ImagesDir: job.ImagesDir,
		ImageAt: nil, Icons: icons, CutOff: job.CutOff,
	}
	var sent *recorder
	switch {
	case job.ImagesRoot != "":
		root := files.NewRoot(job.ImagesRoot)
		defer root.Close() //nolint:errcheck // only read from
		o.Images = root
	case job.Sent != nil:
		fsys := fstest.MapFS{}
		for name, b := range job.Sent {
			fsys[name] = &fstest.MapFile{Data: b, Mode: 0, ModTime: time.Time{}, Sys: nil}
		}
		sent = newRecorder(fsys)
		o.Images, o.ImageAt = sent, sent.at
	}
	page, warnings, err := tmpl.Draw(k, job.File, job.Source, o)
	r := Result{Page: page, Warnings: warnings, Err: err, Read: nil, Places: nil}
	if sent != nil {
		r.Read, r.Places = sent.names, sent.places
	}
	return r
}

// Here is a Drawer that draws with k and icons in the process it runs in, unwatched: for the tests of what the
// server does, which floe __draw would do the same.
func Here(k *kit.Kit, icons diagram.Icons) Drawer {
	return here{k: k, icons: icons}
}

type here struct {
	k     *kit.Kit
	icons diagram.Icons
}

func (h here) Start(ctx context.Context) Session {
	return &hereSession{h: h, ctx: ctx}
}

type hereSession struct {
	h   here
	ctx context.Context //nolint:containedctx // the session's, which ends its drawings
}

func (s *hereSession) Draw(job Job) Result {
	if err := s.ctx.Err(); err != nil {
		return failed(err)
	}
	return One(s.h.k, s.h.icons, job)
}

func (*hereSession) Close() {}

// failed is a Result of floe's failure err.
func failed(err error) Result {
	return Result{Page: nil, Warnings: nil, Err: err, Read: nil, Places: nil}
}

// recorder notes each name read of fsys, with Stat as with Open: kit.ReadImage stats an image before it opens it, and
// an image that is not there fails the Stat. It notes too where the data file names each.
type recorder struct {
	fsys   fs.FS
	names  []string
	seen   map[string]bool
	places map[string][]yamlpos.Pos
}

func newRecorder(fsys fs.FS) *recorder {
	return &recorder{fsys: fsys, names: nil, seen: map[string]bool{}, places: map[string][]yamlpos.Pos{}}
}

func (r *recorder) at(name string, line, column int) {
	r.places[name] = append(r.places[name], yamlpos.Pos{Line: line, Column: column})
}

func (r *recorder) note(name string) {
	if !r.seen[name] {
		r.seen[name] = true
		r.names = append(r.names, name)
	}
}

func (r *recorder) Open(name string) (fs.File, error) {
	r.note(name)
	return r.fsys.Open(name) //nolint:wrapcheck // as fsys says
}

func (r *recorder) Stat(name string) (fs.FileInfo, error) {
	r.note(name)
	return fs.Stat(r.fsys, name) //nolint:wrapcheck // as fsys says
}

// errEnded is the failure of a session whose context ended.
var errEnded = errors.New("the drawing was called off")
