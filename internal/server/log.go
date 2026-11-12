package server

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"

	"floe/internal/files"
	"floe/internal/usererr"
)

// LogName is the file in the store's directory that the server writes what happens to it in: starting, stopping
// and why, a request that panicked, and what brought it down.
const LogName = "server.log"

// logCap is the size over which OpenLog moves the log aside.
const logCap = 1 << 20

// OpenLog opens the server's log, LogName under root, to append to, and returns it: files.OpenAppend refuses what is
// not a regular file, and one over logCap is moved to LogName+".1" first, which replaces the generation before. It does
// nothing else (debug.SetCrashOutput is for serve to call, once), so that opening it in a test changes nothing of the
// process.
// LIMIT: the size is looked at on opening only, so a server that runs on and panics on passes the cap; and two servers
// of one store (on two ports) that start at once with the log over the cap race between looking and moving: one's
// first line may land in LogName+".1", or the generation before be lost. Move it under the store's lock if that
// comes to matter.
func OpenLog(root string) (*os.File, error) {
	name := filepath.Join(root, LogName)
	f, info, err := files.OpenAppend(name)
	if err != nil {
		return nil, usererr.CannotWrite(name, err) //nolint:wrapcheck // the system's words, at the file
	}
	if info.Size() <= logCap {
		return f, nil
	}
	_ = f.Close() //nolint:errcheck // nothing written
	// A link is moved itself, not what it leads to.
	if err = os.Rename(name, name+".1"); err != nil {
		return nil, usererr.CannotWrite(name+".1", err) //nolint:wrapcheck // as above
	}
	if f, _, err = files.OpenAppend(name); err != nil {
		return nil, usererr.CannotWrite(name, err) //nolint:wrapcheck // as above
	}
	return f, nil
}

// newLog is the logger of a server that writes to w: a line of key=value to a thing that happens, its time first and
// this process's pid in it, as two servers of one store (two ports) share the file.
func newLog(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil)).With(slog.Int("pid", os.Getpid()))
}

// errPanic is what a request that panicked is answered with, before where and why.
var errPanic = errors.New("panic")

// recovered records a request that panics, with its stack, and answers it with floe's failure (500), where net/http
// would close the connection and say nothing. http.ErrAbortHandler, a handler's own way to cut a response short, goes
// on to net/http, which closes the connection quietly.
// LIMIT: a panic after the response began answers into what was sent, and net/http tells of it in the log as a
// second WriteHeader; follow what is written with a ResponseWriter of our own if that comes to matter.
func (s *Server) recovered(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v) //nolint:forbidigo // net/http's own way to cut a response short: it is net/http's to take
			}
			s.log.Error("panic in request", slog.String("method", r.Method), slog.String("path", r.URL.Path),
				slog.Any("panic", v), slog.String("stack", string(debug.Stack())))
			refuse(w, r, fmt.Errorf("%w in %s %s: %v; the stack is in %s", errPanic, r.Method, r.URL.Path, v,
				filepath.Join(s.cfg.Store.Root(), LogName)))
		}()
		next.ServeHTTP(w, r)
	})
}
