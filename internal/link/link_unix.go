//go:build unix

package link

// hidden reports whether the file or directory at path is hidden by an attribute: Unix has none (a dot is a name).
func hidden(string) (bool, error) { return false, nil }

// appDataDirs is where the system keeps applications' data of this user: Unix names no such folders.
func appDataDirs() []appDir { return nil }
