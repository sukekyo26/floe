package store

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"floe/internal/files"
	"floe/internal/usererr"
)

const settingsName = "settings.json"

// Settings is settings.json: what the management page's settings set for the whole store.
type Settings struct {
	// Theme is the default theme a page takes when it is first stored (Meta.CreatedTheme), "" for none chosen: then
	// floe's default. Choosing floe's default writes "default", so that chosen and not chosen stay apart.
	Theme string `json:"theme,omitzero"`
}

func (s *Store) settingsFile() string {
	return filepath.Join(s.root, settingsName)
}

// ReadSettings reads settings.json, none being none chosen. What cannot be read is the system's words at the file;
// what encoding/json/v2 refuses (a key twice, a key it does not know, a theme that is not text) is its words there.
// It takes no lock: SetTheme replaces the file whole.
func (s *Store) ReadSettings() (Settings, error) {
	var set Settings
	b, _, err := files.Read(s.settingsFile())
	if errors.Is(err, fs.ErrNotExist) {
		return set, nil
	}
	if err != nil {
		return set, usererr.CannotRead(s.settingsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	if err := jsonv2.Unmarshal(b, &set, jsonv2.RejectUnknownMembers(true)); err != nil {
		return Settings{}, usererr.LibraryAt(s.settingsFile(), 0, 0, err.Error())
	}
	return set, nil
}

// SetTheme writes theme as the default theme of settings.json, whole and without reading it, so that a file that
// cannot be read is mended by choosing again. The caller checks theme is one.
func (s *Store) SetTheme(theme string) error {
	unlock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock()
	b, err := jsonv2.Marshal(Settings{Theme: theme})
	if err != nil {
		return fmt.Errorf("encode %s: %w", settingsName, err)
	}
	if err := files.WriteAtomic(s.settingsFile(), append(b, '\n'), 0o600, nil); err != nil {
		return usererr.CannotWrite(s.settingsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	return nil
}
