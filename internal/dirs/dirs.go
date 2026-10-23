// Package dirs tells what floe needs to know of a directory the user set, before it writes there.
package dirs

import "os"

// CheckWritable makes dir, and a directory in it named after prefix, which it removes at once: a directory there
// already that floe cannot write in passes MkdirAll, so making one in it is what tells.
func CheckWritable(dir, prefix string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err //nolint:wrapcheck // the caller tells where it meant to write
	}
	tmp, err := os.MkdirTemp(dir, prefix)
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	return os.Remove(tmp) //nolint:wrapcheck // as above
}
