package files

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// openFlags is none: the system's open ignores the Unix flags, and opening a named pipe does not block.
	openFlags = 0
	// noFollow opens a reparse point itself rather than where it leads, so that a file made anew is not made there.
	noFollow = syscall.FILE_FLAG_OPEN_REPARSE_POINT
)

// Lock locks f, a file of its own, for this process alone, waiting while another holds it, and returns what lets
// the lock go and closes f: the lock is let go first, which closing may not do at once (LockFileEx). The system's
// error is returned as it is, for the caller to tell at the file.
func Lock(f *os.File) (func(), error) {
	h := windows.Handle(f.Fd())
	err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, ^uint32(0), ^uint32(0), new(windows.Overlapped))
	if err != nil {
		return nil, err //nolint:wrapcheck // the system's words, for the caller to tell
	}
	return func() {
		//nolint:errcheck // closing lets it go too
		_ = windows.UnlockFileEx(h, 0, ^uint32(0), ^uint32(0), new(windows.Overlapped))
		_ = f.Close() //nolint:errcheck // as above
	}, nil
}

// fileBasicInfo is FILE_BASIC_INFO, which x/sys has no type of: as Go's internal/syscall/windows lays it
// out.
type fileBasicInfo struct {
	CreationTime   int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangedTime    int64
	FileAttributes uint32
	_              uint32 // padding to 8 bytes
}

// fileIDInfo is FILE_ID_INFO, which x/sys has no type of.
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

// Mark is what the file at path is now, to tell when it changes: its attributes, ID, size, last write time and
// change time, of the file itself, and of what it leads to for a reparse point (a symlink, a junction); "-" for none.
// The change time tells what a write time set back would hide, the ID a file replaced by a rename.
// LIMIT: the file is opened (for its attributes alone) to be marked, every second for one's own directories; a
// rename over it at that moment (an editor's save) fails. Open it as os.Root does, sharing delete, if that comes up.
func Mark(path string) string {
	f, ok := statOf(path, windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if !ok {
		return "-"
	}
	m := stampOf(f)
	if f.attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		to, ok := statOf(path, 0)
		if !ok {
			return m + ">-"
		}
		m += ">" + stampOf(to)
	}
	return m
}

// noFields is the fields of no file.
var noFields fields //nolint:gochecknoglobals // a zero value, never written

// infoEx reads the class of information of h into the n bytes at p.
func infoEx(h windows.Handle, class uint32, p unsafe.Pointer, n uintptr) error {
	//nolint:wrapcheck,gosec // the system's; G115: a struct's size
	return windows.GetFileInformationByHandleEx(h, class, (*byte)(p), uint32(n))
}

// statOf tells the fields of path, opened with the extra flags, and false when it cannot be opened or told.
func statOf(path string, flags uint32) (fields, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return noFields, false
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|flags, 0)
	if err != nil {
		return noFields, false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read only
	var bh windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &bh) != nil {
		return noFields, false
	}
	f := fields{
		dir:    bh.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0,
		attrs:  bh.FileAttributes,
		id:     fmt.Sprintf("%x:%x%08x", bh.VolumeSerialNumber, bh.FileIndexHigh, bh.FileIndexLow),
		size:   int64(bh.FileSizeHigh)<<32 | int64(bh.FileSizeLow),
		write:  bh.LastWriteTime.Nanoseconds(),
		change: 0, hasChange: false,
	}
	var basic fileBasicInfo
	if infoEx(h, windows.FileBasicInfo, unsafe.Pointer(&basic), unsafe.Sizeof(basic)) == nil {
		f.change, f.hasChange = basic.ChangedTime, true
	}
	var id fileIDInfo
	if infoEx(h, windows.FileIdInfo, unsafe.Pointer(&id), unsafe.Sizeof(id)) == nil {
		f.id = fmt.Sprintf("%x:%x", id.VolumeSerialNumber, id.FileID)
	}
	return f, true
}
