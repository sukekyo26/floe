package files

import (
	"fmt"
	"io/fs"
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
	// checksPaths: allowed looks at a path before the system is called (shares, devices and pipes by name).
	checksPaths = true
)

// driveType is the kind of drive at root (X:\), as GetDriveType tells it.
func driveType(root string) uint32 {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return driveUnknown
	}
	return windows.GetDriveType(p)
}

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

// stampAt is the stamp of the file at path, of the file itself, or of what it leads to when follow (a symlink, a
// junction): its attributes, ID, size, last write time and change time. The change time tells what a write time
// set back would hide, the ID a file replaced by a rename. It tells whether the file is a reparse point, and false
// for none there.
// LIMIT: the file is opened (for its attributes alone) to be marked, every second for one's own directories; an
// os.Rename (MoveFileEx) over it at that moment (an editor's save) fails, though it is opened sharing delete, as it
// does over an os.Stat or os.Lstat (looked at without a pause, 495 of 2000 such saves failed by Mark, 525 by Stat).
// GetFileInformationByName stops none but is in Windows 11 24H2 on only, and allowed's Lstat stops as many. Only the
// renamer can mend it, by renaming with POSIX semantics (ReplaceFile and os.Root do).
func stampAt(path checked, follow bool) (part string, link, ok bool) {
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if follow {
		flags = 0
	}
	f, ok := statOf(path, flags)
	if !ok {
		return "", false, false
	}
	return stampOf(f), f.attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, true
}

// noFields is the fields of no file.
var noFields fields //nolint:gochecknoglobals // a zero value, never written

// infoEx reads the class of information of h into the n bytes at p.
func infoEx(h windows.Handle, class uint32, p unsafe.Pointer, n uintptr) error {
	//nolint:wrapcheck,gosec // the system's; G115: a struct's size
	return windows.GetFileInformationByHandleEx(h, class, (*byte)(p), uint32(n))
}

// openShared opens path to read alone, sharing delete with others, so that a file floe reads may be renamed over and
// its directory removed meanwhile (POSIX semantics, as on Unix); the system's error as os.OpenFile tells it.
func openShared(path checked) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path.s)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path.s, Err: err}
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path.s, Err: err}
	}
	return os.NewFile(uintptr(h), path.s), nil
}

// renameIn renames from to to, two names in dir, with POSIX semantics (os.Root's): over a file another holds open
// sharing delete, where os.Rename (MoveFileEx) is denied.
func renameIn(dir checked, from, to string) error {
	root, err := openRootAt(dir)
	if err != nil {
		return err
	}
	defer root.Close()           //nolint:errcheck // only renamed in
	return root.Rename(from, to) //nolint:wrapcheck // the system's, for the caller to tell
}

// openAttrs opens path for its attributes alone, all shared, a directory too, with the extra flags.
func openAttrs(path checked, flags uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path.s)
	if err != nil {
		return 0, err //nolint:wrapcheck // the caller's to tell
	}
	//nolint:wrapcheck // the system's, for the caller to tell
	return windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|flags, 0)
}

// statOf tells the fields of path, opened with the extra flags, and false when it cannot be opened or told.
func statOf(path checked, flags uint32) (fields, bool) {
	h, err := openAttrs(path, flags)
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

// The flags of GetFinalPathNameByHandle, which x/sys does not name: the name normalized (long names, case as on
// disk) with its drive letter (\\?\C:\…). Both are 0, as Go's internal/syscall/windows has them.
const (
	fileNameNormalized = 0x0
	volumeNameDOS      = 0x0
)

// realOf is the final path of path: opened following its links and junctions, the name the system gives the
// handle, \\?\C:\… or \\?\UNC\host\share\….
func realOf(path checked) (string, error) {
	h, err := openAttrs(path, 0)
	if err != nil {
		return "", &fs.PathError{Op: "open", Path: path.s, Err: err}
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read only
	return grown("GetFinalPathNameByHandle", path.s, func(buf []uint16) (uint32, error) {
		//nolint:gosec // G115: a buffer's length
		return windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), fileNameNormalized|volumeNameDOS)
	})
}

// longNameOf is path with each short name (SECRET~1) made long, as the system names it; links and junctions are
// not followed.
func longNameOf(path checked) (string, error) {
	p, err := windows.UTF16PtrFromString(path.s)
	if err != nil {
		return "", &fs.PathError{Op: "GetLongPathName", Path: path.s, Err: err}
	}
	return grown("GetLongPathName", path.s, func(buf []uint16) (uint32, error) {
		return windows.GetLongPathName(p, &buf[0], uint32(len(buf))) //nolint:gosec // G115: a buffer's length
	})
}

// grown is the name call writes into a buffer, of MAX_PATH first: call gives the name's length, or, when the
// buffer is too short, the length it needs with the closing NUL, for which the buffer is made again.
func grown(op, path string, call func(buf []uint16) (uint32, error)) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := call(buf)
		if err != nil {
			return "", &fs.PathError{Op: op, Path: path, Err: err}
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}
