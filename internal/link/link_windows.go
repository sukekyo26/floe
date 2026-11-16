package link

import (
	"io/fs"

	"golang.org/x/sys/windows"
)

// hidden reports whether the file or directory at path has the hidden attribute, or why it cannot be read.
func hidden(path string) (bool, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, &fs.PathError{Op: "GetFileAttributes", Path: path, Err: err}
	}
	a, err := windows.GetFileAttributes(p)
	if err != nil {
		return false, &fs.PathError{Op: "GetFileAttributes", Path: path, Err: err}
	}
	return a&windows.FILE_ATTRIBUTE_HIDDEN != 0, nil
}

// appDataDirs is where the system keeps applications' data of this user (RoamingAppData, LocalAppData,
// LocalAppDataLow), as it answers this process, or why it cannot tell one. It does not look whether each is there
// (KF_FLAG_DONT_VERIFY): one not there fails as the system's own failure code, which is no fs.ErrNotExist, so that
// files.Real tells it instead.
// LIMIT: a package's own folders are answered to a process run as one (KF_FLAG_NO_PACKAGE_REDIRECTION would not);
// none was seen (S315's W28). Add the flag if floe comes to run as a package.
func appDataDirs() []appDir {
	found := make([]appDir, 0, 3)
	for _, id := range []*windows.KNOWNFOLDERID{
		windows.FOLDERID_RoamingAppData, windows.FOLDERID_LocalAppData, windows.FOLDERID_LocalAppDataLow,
	} {
		dir, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY)
		found = append(found, appDir{path: dir, err: err})
	}
	return found
}
