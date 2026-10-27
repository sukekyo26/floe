package server

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/floe/internal/files"
)

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
		b.WriteString(root + "=" + files.Mark(root) + "\n")
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
			b.WriteString(at + "=" + files.Mark(at) + "\n")
			return nil
		})
	}
	if icons := s.cfg.Icons; icons != "" {
		b.WriteString(icons + "=" + files.Mark(icons) + "\n")
		sets, _ := os.ReadDir(icons) //nolint:errcheck // none to read is in the root's mark
		for _, set := range sets {
			if files.Hidden(set.Name()) {
				continue
			}
			for _, at := range []string{filepath.Join(icons, set.Name()), filepath.Join(icons, set.Name(), "index.json")} {
				b.WriteString(at + "=" + files.Mark(at) + "\n")
			}
		}
	}
	return b.String()
}
