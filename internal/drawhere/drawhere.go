// Package drawhere draws a page in the process it runs in, unwatched: what floe __draw does for each page the server
// asks for, and, for tests alone, a drawproc.Drawer that draws as floe __draw would (depguard's rule
// drawhere-in-guarded keeps the server from it).
package drawhere

import (
	"context"
	"io/fs"
	"testing/fstest"
	"time"

	"floe/internal/diagram"
	"floe/internal/drawproc"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/tmpl"
	"floe/internal/yamlpos"
)

// One draws job with k and the icons in the process it runs in: what floe __draw does for each job, and Here.
func One(k *kit.Kit, icons diagram.Icons, job drawproc.Job) drawproc.Result {
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
	r := drawproc.Result{Page: page, Warnings: warnings, Err: err, Read: nil, Places: nil}
	if sent != nil {
		r.Read, r.Places = sent.names, sent.places
	}
	return r
}

// Here is a Drawer that draws with k and icons in the process it runs in, unwatched: for the tests of what the
// server does, which floe __draw would do the same.
func Here(k *kit.Kit, icons diagram.Icons) drawproc.Drawer {
	return here{k: k, icons: icons}
}

type here struct {
	k     *kit.Kit
	icons diagram.Icons
}

func (h here) Start(ctx context.Context) drawproc.Session {
	return &hereSession{h: h, ctx: ctx}
}

type hereSession struct {
	h   here
	ctx context.Context //nolint:containedctx // the session's, which ends its drawings
}

func (s *hereSession) Draw(job drawproc.Job) drawproc.Result {
	if err := s.ctx.Err(); err != nil {
		return drawproc.Result{Page: nil, Warnings: nil, Err: err, Read: nil, Places: nil}
	}
	return One(s.h.k, s.h.icons, job)
}

func (*hereSession) Close() {}

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
