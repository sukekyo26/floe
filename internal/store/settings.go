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
	// TaskMode is whether floe guide gives agents its Tasks section, which has them ask the user about tasks.
	TaskMode bool `json:"taskMode,omitzero"`
}

// SettingsEdit is a change of settings.json, as the management page's settings send it: each key left as it is when
// nil.
type SettingsEdit struct {
	Theme    *string `json:"theme,omitzero"`
	TaskMode *bool   `json:"taskMode,omitzero"`
}

func (s *Store) settingsFile() string {
	return filepath.Join(s.root, settingsName)
}

// ReadSettings reads settings.json, none being none chosen. What cannot be read is the system's words at the file;
// what encoding/json/v2 refuses (a key twice, a key it does not know, a theme that is not text) is its words there.
// It takes no lock: SetSettings replaces the file whole.
func (s *Store) ReadSettings() (Settings, error) {
	b, err := s.settingsBytes()
	if err != nil {
		return Settings{Theme: "", TaskMode: false}, err
	}
	return s.decodeSettings(b)
}

// settingsBytes is settings.json as it is, nil for none, or the system's words at the file when it cannot be read.
func (s *Store) settingsBytes() ([]byte, error) {
	b, _, err := files.Read(s.settingsFile())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, usererr.CannotRead(s.settingsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	return b, nil
}

// decodeSettings is b as Settings, none for nil, or encoding/json/v2's words at the file, with how to mend it: no
// command writes settings.json, and saving a setting writes it anew (SetSettings).
func (s *Store) decodeSettings(b []byte) (Settings, error) {
	var set Settings
	if b == nil {
		return set, nil
	}
	if err := jsonv2.Unmarshal(b, &set, jsonv2.RejectUnknownMembers(true)); err != nil {
		broken := usererr.LibraryWith("store_settings_broken", err).PlacedAt(s.settingsFile(), 0, 0)
		return Settings{Theme: "", TaskMode: false}, broken
	}
	return set, nil
}

// SetSettings writes the keys ed names to settings.json under the lock, the others as they are, and returns what it
// wrote. A file whose JSON is refused is written anew from ed alone, so that it is mended by choosing again; one the
// system cannot read is refused, and stays as it is, so that a key in it is not lost unseen. The caller checks a theme
// is one.
func (s *Store) SetSettings(ed SettingsEdit) (Settings, error) {
	// Nothing to change writes nothing: a file it cannot read stays, and is told as it is
	if ed.Theme == nil && ed.TaskMode == nil {
		return s.ReadSettings()
	}
	unlock, err := s.lock(true)
	if err != nil {
		return Settings{}, err
	}
	defer unlock()
	b, err := s.settingsBytes()
	if err != nil {
		return Settings{}, err
	}
	set, err := s.decodeSettings(b)
	if err != nil {
		set = Settings{Theme: "", TaskMode: false}
	}
	if ed.Theme != nil {
		set.Theme = *ed.Theme
	}
	if ed.TaskMode != nil {
		set.TaskMode = *ed.TaskMode
	}
	b, err = jsonv2.Marshal(set)
	if err != nil {
		return Settings{}, fmt.Errorf("encode %s: %w", settingsName, err)
	}
	if err := files.WriteAtomic(s.settingsFile(), append(b, '\n'), 0o600, nil); err != nil {
		return Settings{}, usererr.CannotWrite(s.settingsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	return set, nil
}
