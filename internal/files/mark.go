package files

import "fmt"

// Mark is what the file at path is now, to tell when it changes: its stamp (stampAt), and for a link the stamp of
// what it leads to after ">", or ">-" when it leads nowhere; "-" for none there.
func Mark(path string) string {
	m, link, ok := stampAt(path, false)
	if !ok {
		return "-"
	}
	if link {
		to, _, ok := stampAt(path, true)
		if !ok {
			return m + ">-"
		}
		m += ">" + to
	}
	return m
}

// fields are what a mark is made of where the system tells them one by one (Windows), each there or not: a
// filesystem that keeps no change time (FAT) or no 128-bit ID (FAT, some shares) still gives a mark of the rest.
type fields struct {
	dir       bool
	attrs     uint32
	id        string // the file's ID on its volume, "" when the system tells none
	size      int64
	write     int64 // the last write time
	change    int64 // the change time, of the content, the attributes or the times
	hasChange bool
}

// stampOf is f's part of a mark: a directory is its kind and ID alone, as stamp has it; what was not told is left
// empty, so that a mark is never "-" for a file that is there, and one without a change time differs from one of 0.
func stampOf(f fields) string {
	if f.dir {
		return fmt.Sprintf("d%x:%s", f.attrs, f.id)
	}
	change := ""
	if f.hasChange {
		change = fmt.Sprint(f.change)
	}
	return fmt.Sprintf("%x:%s:%d:%d:%s", f.attrs, f.id, f.size, f.write, change)
}
