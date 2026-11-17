package files

import (
	"errors"
	"io/fs"
	"path/filepath"
	"syscall"
)

// ErrNotDir is what a path is told with when what is on its way is a file: the words Unix says.
var ErrNotDir = errors.New("not a directory")

// NotDir is err, of a path, told as a file on its way (ErrNotDir at err's op and path, from for a path when err has
// none), or nil when it is not one. from is where to look up from: the directory of a file read, or the path of a
// directory made or looked at, so that a file put at the name since is not taken for one on the way. Unix says so
// itself (ENOTDIR); Windows says ERROR_PATH_NOT_FOUND, which Go reads both as ENOTDIR and as not there, for a file
// on the way and for a directory not there alike (S315's W12), and only then is it looked up, to the first that is
// there.
// LIMIT: only the branches that told ENOTDIR apart ask it; elsewhere Windows reads a path under a file as not there
// and goes on as if it were (settings.json under a FLOE_DIR that is a file is none, as is a default place of one's
// own that is a file, in ownDir; kit.has). Ask here where that comes to mislead.
func NotDir(from string, err error) error {
	if !errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	op, path := "stat", from
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		op, path = pe.Op, pe.Path
	}
	told := &fs.PathError{Op: op, Path: path, Err: ErrNotDir}
	if !errors.Is(err, fs.ErrNotExist) {
		return told
	}
	for dir := from; ; {
		info, serr := Stat(dir)
		switch {
		case serr == nil && info.IsDir():
			return nil
		case serr == nil:
			return told
		case !errors.Is(serr, fs.ErrNotExist):
			return nil
		}
		up := filepath.Dir(dir)
		if up == dir {
			return nil
		}
		dir = up
	}
}
