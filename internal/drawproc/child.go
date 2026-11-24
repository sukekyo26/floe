package drawproc

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

// Stopped is the exit code of floe __draw stopped by its watch of the memory: neither 1 (Windows' Kill) nor 2 (Go's
// own fall).
const Stopped = 3

// TimedOut is the exit code of floe __draw that drew one page past ChildLimit, its server gone or late to end it.
const TimedOut = 4

// ChildLimit is how long floe __draw draws one page before it ends itself: past Limit, by when its server ends it.
const ChildLimit = Limit + 10*time.Second

// Children is how many floe __draw the server runs at once, and so twice the memory one may hold, at most.
const Children = 2

// The most of an answer read (a page of tmpl.MaxPage, 80 MiB, is 107 MiB in JSON's base64), and of the stderr of a
// child that failed kept for its failure.
const (
	answerLimit = 128 << 20
	stderrTail  = 4 << 10
)

// request is a job as the server hands it to floe __draw, with the build of the server, which a floe installed
// again since the server started does not draw for.
type request struct {
	Build string `json:"build"`
	Job   Job    `json:"job"`
}

// answer is a Result as floe __draw hands it back.
type answer struct {
	Page     []byte                   `json:"page,omitzero"`
	Warnings []usererr.Copied         `json:"warnings,omitzero"`
	Mistakes []usererr.Copied         `json:"mistakes,omitzero"`
	Failure  string                   `json:"failure,omitzero"`
	Read     []string                 `json:"read,omitzero"`
	Places   map[string][]yamlpos.Pos `json:"places,omitzero"`
}

// answerOf is r as an answer: its mistakes copied, or floe's failure as text.
// LIMIT: one argument usererr.Copy cannot copy makes the whole page floe's failure; the catalog's arguments are texts
// and whole numbers now, which no test holds every key to. Copy the kind if one comes.
func answerOf(r Result) answer {
	a := answer{Page: r.Page, Warnings: nil, Mistakes: nil, Failure: "", Read: r.Read, Places: r.Places}
	var mistakes usererr.List
	err := mistakes.Add(r.Err)
	for _, e := range mistakes {
		if err != nil {
			break
		}
		var c usererr.Copied
		if c, err = usererr.Copy(e); err == nil {
			a.Mistakes = append(a.Mistakes, c)
		}
	}
	for _, e := range r.Warnings {
		if err != nil {
			break
		}
		var c usererr.Copied
		if c, err = usererr.Copy(e); err == nil {
			a.Warnings = append(a.Warnings, c)
		}
	}
	if err != nil {
		return answer{Page: nil, Warnings: nil, Mistakes: nil, Failure: err.Error(), Read: nil, Places: nil}
	}
	return a
}

// result is the Result a hands back.
func (a answer) result() Result {
	r := Result{Page: a.Page, Warnings: nil, Err: nil, Read: a.Read, Places: a.Places}
	if a.Failure != "" {
		r.Err = errors.New(a.Failure) //nolint:err113 // floe's failure, as the child worded it
		return r
	}
	for _, c := range a.Warnings {
		r.Warnings = append(r.Warnings, c.Error())
	}
	var mistakes usererr.List
	for _, c := range a.Mistakes {
		mistakes = append(mistakes, c.Error())
	}
	r.Err = mistakes.Err()
	return r
}

// Serve is floe __draw: it reads requests from r until it ends, and writes to w the answer of each, drawn by draw;
// build is its own, and a request of another is refused (draw_build). A page drawn longer than limit ends the
// process with exit(TimedOut), as the server would end it, for one whose server is gone: the child reads that only
// between pages, where r ends.
func Serve(
	r io.Reader, w io.Writer, build string, limit time.Duration, exit func(code int), draw func(Job) Result,
) error {
	dec, enc := jsontext.NewDecoder(r), jsontext.NewEncoder(w)
	for {
		var req request
		if err := jsonv2.UnmarshalDecode(dec, &req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read what to draw: %w", err)
		}
		res := failed(usererr.NewWhole("draw_build"))
		if req.Build == build {
			timer := time.AfterFunc(limit, func() { exit(TimedOut) })
			res = draw(req.Job)
			timer.Stop()
		}
		if err := jsonv2.MarshalEncode(enc, answerOf(res)); err != nil {
			return fmt.Errorf("write what was drawn: %w", err)
		}
	}
}

// Child is the Drawer of the server: each session runs floe __draw, command, one more after one that stopped, and
// ends it when it takes longer than Limit for a page; Children sessions at once.
// LIMIT: a session holds its slot until it is closed: a rebuild holds one until it ends or its command is stopped, and
// two pages that never end hold both for Limit, every other drawing (a theme's sample too) waiting. Give a rebuild a
// slot a page if that comes to matter.
type Child struct {
	build   string
	command func(ctx context.Context) *exec.Cmd
	slots   chan struct{}
	limit   time.Duration
	after   func(d time.Duration, f func()) stopper // time.AfterFunc, another in a test
}

// stopper is a timer, which Stop stops before it goes off (true) or after (false).
type stopper interface {
	Stop() bool
}

// afterFunc is time.AfterFunc as a stopper.
func afterFunc(d time.Duration, f func()) stopper {
	return time.AfterFunc(d, f)
}

// NewChild is the Child of the server of build, which runs floe __draw by command, the process ended with ctx, and
// ends it when it draws one page longer than limit (Limit).
func NewChild(build string, limit time.Duration, command func(ctx context.Context) *exec.Cmd) *Child {
	return &Child{build: build, command: command, slots: make(chan struct{}, Children), limit: limit, after: afterFunc}
}

// Start waits for one of the Children slots, or for ctx to end.
func (c *Child) Start(ctx context.Context) Session {
	select {
	case c.slots <- struct{}{}:
		return &session{c: c, ctx: ctx, proc: nil, held: true}
	case <-ctx.Done():
		return &session{c: c, ctx: ctx, proc: nil, held: false}
	}
}

type session struct {
	c    *Child
	ctx  context.Context //nolint:containedctx // the session's, which ends its processes
	proc *proc
	held bool
}

// proc is a floe __draw running.
type proc struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	in       io.WriteCloser
	enc      *jsontext.Encoder
	dec      *jsontext.Decoder
	out      *capped
	stderr   *tail
	timedOut atomic.Bool
}

func (s *session) Draw(job Job) Result {
	if !s.held {
		return failed(errEnded)
	}
	if s.proc == nil {
		p, err := s.c.run(s.ctx)
		if err != nil {
			return failed(err)
		}
		s.proc = p
	}
	p := s.proc
	p.out.left = answerLimit
	timer := s.c.after(s.c.limit, func() {
		p.timedOut.Store(true)
		p.cancel()
	})
	err := jsonv2.MarshalEncode(p.enc, request{Build: s.c.build, Job: job})
	var a answer
	if err == nil {
		err = jsonv2.UnmarshalDecode(p.dec, &a)
	}
	if timer.Stop() && err == nil {
		return a.result()
	}
	s.proc = nil
	if err == nil {
		// The time ran out as the answer came: the answer stands, and the child, ended or being ended, goes.
		p.end(s.ctx, job.File, s.c.limit, nil) //nolint:errcheck // the page was drawn
		return a.result()
	}
	return failed(p.end(s.ctx, job.File, s.c.limit, err))
}

func (s *session) Close() {
	if s.proc != nil {
		_ = s.proc.in.Close() //nolint:errcheck // the child ends at its end, or is ended
		_ = s.proc.cmd.Wait() //nolint:errcheck // done with it
		s.proc.cancel()
	}
	if s.held {
		<-s.c.slots
		s.held = false
	}
}

// run starts floe __draw, which ctx ends.
func (c *Child) run(ctx context.Context) (*proc, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := c.command(ctx)
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start floe __draw: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start floe __draw: %w", err)
	}
	stderr := &tail{mu: sync.Mutex{}, b: nil}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start floe __draw: %w", err)
	}
	out := &capped{r: stdout, left: answerLimit}
	return &proc{
		cmd: cmd, cancel: cancel, in: in, enc: jsontext.NewEncoder(in), dec: jsontext.NewDecoder(out), out: out,
		stderr: stderr, timedOut: atomic.Bool{},
	}, nil
}

// end ends p, which failed with err as it drew file, and says why: past Limit or its memory, the user's mistake at
// file; else floe's failure, with the end of what it wrote on stderr.
func (p *proc) end(ctx context.Context, file string, limit time.Duration, err error) error {
	p.cancel()
	_ = p.in.Close() //nolint:errcheck // ended
	waited := p.cmd.Wait()
	var exited *exec.ExitError
	switch {
	case p.timedOut.Load(), errors.As(waited, &exited) && exited.ExitCode() == TimedOut:
		return usererr.NewAt(file, 0, 0, "draw_time", int(limit/time.Second))
	case errors.As(waited, &exited) && exited.ExitCode() == Stopped:
		return usererr.NewAt(file, 0, 0, "draw_memory", fmt.Sprintf("%d MiB", memoryLimit>>20))
	case ctx.Err() != nil:
		return errEnded
	case errors.Is(err, errTooLarge):
		return fmt.Errorf("draw %s: %w", file, errTooLarge)
	}
	if waited == nil {
		return fmt.Errorf("draw %s: floe __draw answered what cannot be read: %w: %s", file, err, p.stderr.String())
	}
	return fmt.Errorf("draw %s: floe __draw ended (%w): %w: %s", file, waited, err, p.stderr.String())
}

// errTooLarge is the failure of an answer past answerLimit.
var errTooLarge = fmt.Errorf("the page drawn is over %d MiB", answerLimit>>20)

// capped reads r, up to left bytes, then fails with errTooLarge.
type capped struct {
	r    io.Reader
	left int64
}

func (c *capped) Read(b []byte) (int, error) {
	if c.left <= 0 {
		return 0, errTooLarge
	}
	if int64(len(b)) > c.left {
		b = b[:c.left]
	}
	n, err := c.r.Read(b)
	c.left -= int64(n)
	return n, err //nolint:wrapcheck // as r says
}

// tail keeps the last stderrTail bytes written to it.
type tail struct {
	mu sync.Mutex
	b  []byte
}

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, b...)
	if len(t.b) > stderrTail {
		t.b = t.b[len(t.b)-stderrTail:]
	}
	return len(b), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}
