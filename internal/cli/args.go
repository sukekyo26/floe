package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"floe/internal/dirs"
	"floe/internal/drawproc"
	"floe/internal/files"
	"floe/internal/guard"
	"floe/internal/usererr"
)

// app is what the commands share: the process's environment, and where they write.
type app struct {
	env Env
	out io.Writer
	// The warnings of the pages drawn here, and those the server told of the pages it drew: Run tells them.
	warnings []*usererr.Error
	told     []usererr.Told
	guard    *guard.Guard    // the watch of the memory, from the first page drawn; nil without Env.Exit
	stopCode int             // the exit code past the watch's limit: exitFail, or floe __draw's
	drawer   *drawproc.Child // what the server draws with, set by serve
}

// print writes a line to the command's output.
func (a *app) print(format string, args ...any) {
	_, _ = fmt.Fprintf(a.out, format+"\n", args...) //nolint:errcheck // Run tells what a write failed at
}

// write writes b to the command's output as it is.
func (a *app) write(b []byte) {
	_, _ = a.out.Write(b) //nolint:errcheck // Run tells what a write failed at
}

// getenv is the value of the environment variable key, "" when it is not set.
func (a *app) getenv(key string) string {
	if a.env.Getenv == nil {
		return ""
	}
	return a.env.Getenv(key)
}

// arg is a path a command was given: its name as it was written, which its mistakes and its page go by, and
// where it is, or why it is nowhere.
type arg struct {
	name, abs string
	err       error // an empty path, or a relative path under no working directory; abs is then ""
}

// place is where a command takes a path: its name, as a mistake names it (--out-dir, argument 2), and the
// key of what to give there.
type place struct {
	name, wants string
}

// arg resolves path for the command, once: under the working directory (dirs.Under), as written, so that its ..
// leads where the system says, and every place reads a path alike; a relative one is refused when the working directory
// cannot be read (Env.Dir ""). "" is refused as empty, as a shell variable left unset gives it and it names
// no file. A name of spaces alone is a file of that name.
func (a *app) arg(path string, at place) arg {
	if path == "" {
		return arg{name: path, abs: "", err: usererr.New("cli_path_empty", at.name, english.Msg(at.wants))}
	}
	abs, err := dirs.Under(a.env.Dir, path)
	if errors.Is(err, dirs.ErrDriveRelative) {
		vol := filepath.VolumeName(path)
		return arg{name: path, abs: "", err: usererr.New("path_drive_relative", path, vol, vol)}
	}
	if err != nil {
		return arg{name: path, abs: "", err: usererr.New("cli_relative_no_cwd", path)}
	}
	return arg{name: path, abs: abs, err: nil}
}

// args is the arg of each of the files given, in order, each once (once), each wants (the key of what to
// give) at its place.
func (a *app) args(paths []string, wants string) []arg {
	given := make([]arg, len(paths))
	for i, path := range paths {
		given[i] = a.arg(path, place{name: fmt.Sprintf("argument %d", i+1), wants: wants})
	}
	return once(given)
}

// read reads the file p, or tells why it cannot: in the words of the system, at the file.
func (p arg) read() ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	src, _, err := files.Read(p.abs)
	if err != nil {
		return nil, usererr.CannotRead(p.name, err)
	}
	return src, nil
}

// once is the args given, in order, each once, by the path it was first given as: two paths that name one
// entry of one directory (./a.yaml and a.yaml, through a symlinked directory) are read and told once. A link of
// another name or in another directory, symbolic or hard, is one of its own, as its name names its page; so is a
// name that differs only in case.
// LIMIT: each file is compared with each kept; enough for the files of a command line.
func once(given []arg) []arg {
	var named []arg
	var kept []entry
next:
	for _, p := range given {
		e := entryOf(p)
		for _, k := range kept {
			// Empty, it names nothing for another to be the same as: each is told by its place.
			if p.name != "" && e.same(k) {
				continue next
			}
		}
		named, kept = append(named, p), append(kept, e)
	}
	return named
}

// entry is a path as a name in a directory: its absolute path, and its directory, nil when it cannot be read.
type entry struct {
	abs string
	dir os.FileInfo
}

func entryOf(p arg) entry {
	if p.err != nil {
		// Nowhere, by its name: the command refuses it, and nothing is looked up for it.
		return entry{abs: p.name, dir: nil}
	}
	e := entry{abs: p.abs, dir: nil}
	if dir, err := files.Stat(dirs.Dir(e.abs)); err == nil {
		e.dir = dir
	}
	return e
}

// same reports whether e and o name one entry of one directory: one directory (os.SameFile), and one name, letter
// for letter, case too. When either directory cannot be read, one absolute path. Whether two names are one file
// is left to what writes it (render's targetAt asks the system).
// LIMIT: on a file system that ignores case (macOS, a casefold directory on Linux, exFAT, WSL's drvfs), one file
// named twice in two cases is read as two, as names in NFC and NFD are: check tells its mistakes and warnings
// twice, diagram render draws it twice, and render, when its page is not there yet, draws it twice and writes it
// again only when the titles come from the names and differ, else refuses with cli_render_same_file, whose "rename
// one of them" does not fit. Read the directory's names to ask the system if such command lines come up.
func (e entry) same(o entry) bool {
	if e.dir == nil || o.dir == nil {
		return e.abs == o.abs
	}
	return os.SameFile(e.dir, o.dir) && filepath.Base(e.abs) == filepath.Base(o.abs)
}
