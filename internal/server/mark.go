package server

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/files"
)

// mark is what the file at path is now, to tell when it changes: its kind, inode, size, mtime and ctime, as
// lstat tells them, and as stat tells them for a symlink (what it leads to); "-" for none. ctime and the inode
// tell what a mtime set back (touch -r, cp -p) or a file replaced by a rename would hide.
// LIMIT: a filesystem of times in whole seconds (FAT, some SMB or NFS) hides two writes of one size in one second
// once the first was marked. Add a hash of the content if pages come to be kept on one.
func mark(path string) string {
	var st unix.Stat_t
	if unix.Lstat(path, &st) != nil {
		return "-"
	}
	m := stamp(&st)
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		if unix.Stat(path, &st) != nil {
			return m + ">-"
		}
		m += ">" + stamp(&st)
	}
	return m
}

func stamp(st *unix.Stat_t) string {
	return fmt.Sprintf("%o:%d:%d:%d:%d", st.Mode, st.Ino, st.Size, st.Mtim.Nano(), st.Ctim.Nano())
}

// marks is what the directories of the drawing are now: those of one's own parts, themes and templates, each
// walked whole as kit reads it (os.DirFS, so that one that is a symlink is walked into, and leaving the names
// files.Hidden leaves), and that of the icon sets, with each set and its index.json: floe icons fetch writes a set
// to a directory of its own and renames it in.
// LIMIT: an icon of a set edited in place is not seen: only the sets and their index.json are. Walk the sets too
// if icons come to be edited by hand.
func (s *Server) marks() string {
	var b strings.Builder
	for _, root := range []string{s.cfg.Parts, s.cfg.Themes, s.cfg.Templates} {
		if root == "" {
			continue
		}
		b.WriteString(root + "=" + mark(root) + "\n")
		//nolint:errcheck // what cannot be walked is left out, its directory's mark telling it changed
		_ = fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
			switch {
			case err != nil || p == ".":
				return nil //nolint:nilerr // as above
			case files.Hidden(d.Name()):
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			at := filepath.Join(root, filepath.FromSlash(p))
			b.WriteString(at + "=" + mark(at) + "\n")
			return nil
		})
	}
	if icons := s.cfg.Icons; icons != "" {
		b.WriteString(icons + "=" + mark(icons) + "\n")
		sets, _ := os.ReadDir(icons) //nolint:errcheck // none to read is in the root's mark
		for _, set := range sets {
			if files.Hidden(set.Name()) {
				continue
			}
			for _, at := range []string{filepath.Join(icons, set.Name()), filepath.Join(icons, set.Name(), "index.json")} {
				b.WriteString(at + "=" + mark(at) + "\n")
			}
		}
	}
	return b.String()
}
