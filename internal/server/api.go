package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing/fstest"
	"time"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

const maxBody = 64 << 10

// maxUpload is the size of a publish of a data file, its images in it as base64.
// LIMIT: 32 MiB a publish, while an image may be 1 MiB and a page may show any number of them. Raise
// it, or send the images apart, if pages come to need more.
const maxUpload = 32 << 20

var jsonTypeRe = regexp.MustCompile(`(?i)^application/json\b`)

// readBody reads a write from the management page or the CLI, of up to limit bytes, as JSON. Other
// sites and the sandboxed pages are refused by the exact Origin and by requiring JSON (which needs a
// CORS preflight). On refusal it has answered and returns false.
// LIMIT: 127.0.0.1 is open to every user of the machine, and anyone can fake Origin. Do not run floe
// on a shared machine.
func (s *Server) readBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, bool) {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return nil, false
	}
	if !jsonTypeRe.MatchString(r.Header.Get("Content-Type")) {
		send(w, http.StatusUnsupportedMediaType, "send application/json")
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		send(w, http.StatusRequestEntityTooLarge, "request body too large")
		return nil, false
	}
	if err != nil || !json.Valid(body) {
		send(w, http.StatusBadRequest, catalog(r).Msg("api_not_json"))
		return nil, false
	}
	return body, true
}

// readJSON reads a write of the management page as readBody does, as a JSON object; nil on refusal.
func (s *Server) readJSON(w http.ResponseWriter, r *http.Request) map[string]json.RawMessage {
	body, ok := s.readBody(w, r, maxBody)
	if !ok {
		return nil
	}
	// Anything but an object (a number, a string, an array, null) is a mistake, not an empty change.
	var input map[string]json.RawMessage
	if json.Unmarshal(body, &input) != nil || input == nil {
		send(w, http.StatusBadRequest, catalog(r).Msg("api_not_object"))
		return nil
	}
	return input
}

// refuse answers a usererr with its reason in the reader's language, 409 for a name in use; anything
// else is 500. A reader that takes JSON (the CLI) gets {"message", "mistakes"}, mistakes set for those
// of a data file at their lines and columns, which it shows as they are.
func refuse(w http.ResponseWriter, r *http.Request, err error) {
	var ue *usererr.Error
	// The mistakes of a data file, at their lines and columns, are worded too.
	var l i18n.Localizer
	status, message := http.StatusInternalServerError, err.Error()
	switch {
	case errors.As(err, &ue) && ue.Conflict:
		status, message = http.StatusConflict, ue.Localize(catalog(r))
	case errors.As(err, &l):
		status, message = http.StatusBadRequest, l.Localize(catalog(r))
	}
	if !strings.Contains(r.Header.Get("Accept"), "application/json") {
		send(w, status, message)
		return
	}
	var all yamlpos.Problems
	var one *yamlpos.Error
	sendJSON(w, status, refusal{Message: message, Mistakes: errors.As(err, &all) || errors.As(err, &one)})
}

type refusal struct {
	Message  string `json:"message"`
	Mistakes bool   `json:"mistakes"`
}

type slugBody struct {
	Slug string `json:"slug"`
}

// addLink registers a linked file, from the management page or floe publish --link. A field sent is
// given, empty or not; one left out is not. The management page sends no force: a URL name another file
// uses is refused (409) so that a typo never replaces another link.
func (s *Server) addLink(w http.ResponseWriter, r *http.Request) {
	input := s.readJSON(w, r)
	if input == nil {
		return
	}
	path := text(input["path"])
	if path == nil || !filepath.IsAbs(*path) {
		send(w, http.StatusBadRequest, catalog(r).Msg("api_link_path"))
		return
	}
	var opts store.Options
	if !stringFields(input, map[string]**string{
		"slug": &opts.Slug, "title": &opts.Title, "description": &opts.Description, "theme": &opts.Theme,
		"partsTheme": &opts.PartsTheme,
	}) {
		send(w, http.StatusBadRequest, "send slug, title, description, theme and partsTheme as strings")
		return
	}
	if raw, ok := input["force"]; ok {
		if opts.Force, ok = decode[bool](raw); !ok {
			send(w, http.StatusBadRequest, "send force as true or false")
			return
		}
	}
	entry, err := s.cfg.Store.PrepareLink(r.Context(), *path, opts, filepath.Dir(*path))
	if err != nil {
		refuse(w, r, err)
		return
	}
	if err := s.cfg.Store.Save(entry); err != nil {
		refuse(w, r, err)
		return
	}
	status := http.StatusOK
	if entry.Prev == nil {
		status = http.StatusCreated
	}
	sendJSON(w, status, slugBody{Slug: entry.Slug})
}

// upload is a publish of a data file, as the CLI sends it: see store.Upload. Images left out or null
// asks which images the data names.
type upload struct {
	File        string            `json:"file"`
	Dir         string            `json:"dir"`
	Source      []byte            `json:"source"`
	Images      map[string][]byte `json:"images"`
	Project     string            `json:"project"`
	Title       *string           `json:"title"`
	Description *string           `json:"description"`
	Theme       *string           `json:"theme"`
	PartsTheme  *string           `json:"partsTheme"`
	Force       bool              `json:"force"`
}

// saved is a page a publish stored, and what it replaced.
type saved struct {
	Slug  string     `json:"slug"`
	Title string     `json:"title"`
	Prev  *savedPrev `json:"prev"`
}

type savedPrev struct {
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

// save stores the data file the CLI sends as the page slug: 201 for a new slug, 200 for one replaced,
// and 422 with the images to send when it names some and sent none.
func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	body, ok := s.readBody(w, r, maxUpload)
	if !ok {
		return
	}
	var in upload
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		send(w, http.StatusBadRequest, "cannot read the publish: "+err.Error())
		return
	}
	if in.File == "" || in.Dir == "" || in.Source == nil {
		send(w, http.StatusBadRequest, "send file, dir and source")
		return
	}
	for name := range in.Images {
		if !fs.ValidPath(name) || name == "." {
			send(w, http.StatusBadRequest, "send each image by a slash-separated path under the data file's directory")
			return
		}
	}
	up := store.Upload{File: in.File, Dir: in.Dir, Source: in.Source, Images: imagesFS(in.Images), Project: in.Project}
	opts := store.Options{
		Slug: &slug, Title: in.Title, Description: in.Description, Theme: in.Theme, PartsTheme: in.PartsTheme,
		Force: in.Force,
	}
	entry, err := s.cfg.Store.PrepareData(up, opts)
	var need *store.NeedImages
	if errors.As(err, &need) {
		sendJSON(w, http.StatusUnprocessableEntity, struct {
			Images []string `json:"images"`
		}{Images: need.Names})
		return
	}
	if err == nil {
		err = s.cfg.Store.Save(entry)
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	status := http.StatusOK
	var prev *savedPrev
	if entry.Prev == nil {
		status = http.StatusCreated
	} else {
		prev = &savedPrev{Title: entry.Prev.Title, UpdatedAt: entry.Prev.UpdatedAt}
	}
	sendJSON(w, status, saved{Slug: entry.Slug, Title: entry.Meta.Title, Prev: prev})
}

// imagesFS is the images a publish sent as the data file's directory, nil for none sent.
// LIMIT: fstest.MapFS goes through every name at each step of an image's path, so thousands of images
// in one publish take quadratic time. Index the directories if pages come to show that many.
func imagesFS(images map[string][]byte) fs.FS {
	if images == nil {
		return nil
	}
	files := fstest.MapFS{}
	for name, b := range images {
		files[name] = &fstest.MapFile{Data: b, Mode: 0, ModTime: time.Time{}, Sys: nil}
	}
	return files
}

// text is a non-blank string, trimmed; anything else is nil.
func text(raw json.RawMessage) *string {
	var s string
	if json.Unmarshal(raw, &s) != nil || strings.TrimSpace(s) == "" {
		return nil
	}
	s = strings.TrimSpace(s)
	return &s
}

// stringFields sets each of fields that input sends to its value, trimmed, empty or not, leaving the rest
// nil; false when one sent is not a string.
func stringFields(input map[string]json.RawMessage, fields map[string]**string) bool {
	for key, field := range fields {
		if raw, ok := input[key]; ok {
			v, isString := decode[string](raw)
			if !isString {
				return false
			}
			v = strings.TrimSpace(v)
			*field = &v
		}
	}
	return true
}

// decode reports whether raw is a JSON value of type T (null is none).
func decode[T any](raw json.RawMessage) (T, bool) {
	var v any
	_ = json.Unmarshal(raw, &v) //nolint:errcheck // readJSON has checked the body
	t, ok := v.(T)
	return t, ok
}

// edit changes what the management page sends (title, description, project, URL name, favourite).
func (s *Server) edit(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	input := s.readJSON(w, r)
	if input == nil {
		return
	}
	if s.cfg.Store.ReadMeta(slug) == nil {
		notFound(w)
		return
	}
	var ed store.Edit
	if !stringFields(input, map[string]**string{
		"title": &ed.Title, "description": &ed.Description, "project": &ed.Project, "slug": &ed.Slug,
	}) {
		send(w, http.StatusBadRequest, catalog(r).Msg("api_edit_strings"))
		return
	}
	if raw, ok := input["favorite"]; ok {
		v, isBool := decode[bool](raw)
		if !isBool {
			send(w, http.StatusBadRequest, catalog(r).Msg("api_favorite_bool"))
			return
		}
		ed.Favorite = &v
	}
	to, err := s.cfg.Store.Edit(slug, ed)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, slugBody{Slug: to})
}

// remove deletes a page. Other sites and the sandboxed pages (Origin: null) are refused. It takes no
// body: the management page sends none, so requiring JSON would make deleting impossible.
func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return
	}
	// A concurrent delete that got there first is 404, not 500.
	err := s.cfg.Store.Remove(r.PathValue("slug"))
	var ue *usererr.Error
	if errors.As(err, &ue) {
		notFound(w)
		return
	}
	if err != nil {
		send(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
