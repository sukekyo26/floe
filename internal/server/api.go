package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"

	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

const maxBody = 64 << 10

// MaxUpload is the size of a publish of a data file, its images in it as base64.
// LIMIT: 32 MiB a publish, while an image may be 1 MiB and a page may show any number of them. Raise
// it, or send the images apart, if pages come to need more.
const MaxUpload = 32 << 20

// jsonType is the media type of a write, and the one Accept names for a report as JSON.
const jsonType = "application/json"

// readBody reads a write from the management page or the CLI, of up to limit bytes, as JSON. Other
// sites and the sandboxed pages are refused by the exact Origin and by requiring JSON (which needs a
// CORS preflight), its Content-Type read as mime reads it: one whose parameters cannot be read is refused too.
// On refusal it has answered and returns false.
// LIMIT: 127.0.0.1 is open to every user of the machine, and anyone can fake Origin. Do not run floe
// on a shared machine.
func (s *Server) readBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, bool) {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return nil, false
	}
	if typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || typ != jsonType {
		send(w, http.StatusUnsupportedMediaType, "send application/json")
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		send(w, http.StatusRequestEntityTooLarge, "request body too large")
		return nil, false
	}
	if err != nil || !json.Valid(body) {
		send(w, http.StatusBadRequest, "cannot read the request: it is not JSON")
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
		send(w, http.StatusBadRequest, "cannot read the request: send a JSON object")
		return nil
	}
	return input
}

// refuse answers err told in the reader's language (usererr.Report): 500 for floe's failure, 409 for a name in use
// (store.Conflict, which comes alone), 400 for the user's other mistakes. A reader whose Accept is JSON exactly
// (the CLI) gets the report itself, which it lays out as it does its own mistakes; another, the management page,
// its text.
func refuse(w http.ResponseWriter, r *http.Request, err error) {
	report := usererr.Tell(catalog(r), err)
	status := http.StatusBadRequest
	if _, conflict := errors.AsType[*store.Conflict](err); conflict {
		status = http.StatusConflict
	}
	if report.Failure != "" {
		status = http.StatusInternalServerError
	}
	if r.Header.Get("Accept") != jsonType {
		sendBytes(w, status, textType, []byte(report.Render("")))
		return
	}
	sendJSON(w, status, report)
}

type slugBody struct {
	Slug string `json:"slug"`
}

// Link is a link to add, as the management page and floe publish --link send it: a field sent is given,
// empty or not, and one left out or null is not.
type Link struct {
	Path        string  `json:"path"`
	Slug        *string `json:"slug,omitempty"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Theme       *string `json:"theme,omitempty"`
	Force       bool    `json:"force,omitempty"`
}

// addLink registers a linked file, from the management page or floe publish --link. The management
// page sends no force: a URL name another file uses is refused (409) so that a typo never replaces
// another link.
func (s *Server) addLink(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r, maxBody)
	if !ok {
		return
	}
	var in Link
	if !decodeStrict(w, body, &in) {
		return
	}
	opts := store.Options{Slug: in.Slug, Title: in.Title, Description: in.Description, Theme: in.Theme, Force: in.Force}
	drawing, _ := s.now()
	// The path as sent: the CLI sends it as given, where a file name may start or end with a space (the
	// management page trims it). There is no directory to read a relative one in, which PrepareLink refuses
	// with the other mistakes.
	entry, err := s.cfg.Store.PrepareLink(r.Context(), drawing, in.Path, opts, "", s.cfg.Home)
	if err == nil {
		err = s.cfg.Store.Save(entry)
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendSaved(w, entry)
}

// decodeStrict reads body, a JSON object, into v, refusing a field v does not have. On refusal it has
// answered and returns false.
func decodeStrict(w http.ResponseWriter, body []byte, v any) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		// What encoding/json could not read: neither the management page nor the CLI sends such JSON.
		// LIMIT: in encoding/json's English, as the protocol's refusals are. Word it if a reader comes to see it.
		send(w, http.StatusBadRequest, protocolText("cannot read the request: "+err.Error()))
		return false
	}
	return true
}

// Upload is a publish of a data file, as the CLI sends it: see store.Upload. Images left out or null asks which
// images the data reads.
type Upload struct {
	File        string            `json:"file"`
	Source      []byte            `json:"source"`
	Images      map[string][]byte `json:"images"`
	Project     string            `json:"project"`
	Title       *string           `json:"title"`
	Description *string           `json:"description"`
	Theme       *string           `json:"theme"`
	Force       bool              `json:"force"`
}

// Saved is a page a publish or an added link stored, and what it replaced (nil for a new slug).
type Saved struct {
	Slug  string     `json:"slug"`
	Title string     `json:"title"`
	Prev  *SavedPrev `json:"prev"`
}

// SavedPrev is the page a publish replaced.
type SavedPrev struct {
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

// ImagesNeeded answers a publish without images whose data reads some: the images to send.
type ImagesNeeded struct {
	Images []string `json:"images"`
}

// sendSaved answers the entry a write saved: 201 for a new slug, 200 for one replaced.
func sendSaved(w http.ResponseWriter, entry *store.Entry) {
	status := http.StatusOK
	var prev *SavedPrev
	if entry.Prev == nil {
		status = http.StatusCreated
	} else {
		prev = &SavedPrev{Title: entry.Prev.Title, UpdatedAt: entry.Prev.UpdatedAt}
	}
	sendJSON(w, status, Saved{Slug: entry.Slug, Title: entry.Meta.Title, Prev: prev})
}

// save stores the data file the CLI sends as the page slug: 201 for a new slug, 200 for one replaced,
// and 422 with the images to send when it reads some and sent none.
func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r, MaxUpload)
	if !ok {
		return
	}
	var in Upload
	if !decodeStrict(w, body, &in) {
		return
	}
	if in.File == "" || in.Source == nil {
		send(w, http.StatusBadRequest, "send file and source")
		return
	}
	for name := range in.Images {
		if !fs.ValidPath(name) || name == "." {
			send(w, http.StatusBadRequest, "send each image by a slash-separated path under the data file's directory")
			return
		}
	}
	// The only slug of the publish, which PrepareData checks with the rest: one not to be is the publisher's
	// mistake, not a page not found.
	slug := r.PathValue("slug")
	up := store.Upload{File: in.File, Source: in.Source, Images: in.Images, Project: in.Project}
	opts := store.Options{Slug: &slug, Title: in.Title, Description: in.Description, Theme: in.Theme, Force: in.Force}
	drawing, _ := s.now()
	entry, err := s.cfg.Store.PrepareData(drawing, up, opts)
	if need, ok := errors.AsType[*store.NeedImages](err); ok {
		sendJSON(w, http.StatusUnprocessableEntity, ImagesNeeded{Images: need.Names})
		return
	}
	if err == nil {
		err = s.cfg.Store.Save(entry)
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendSaved(w, entry)
}

// rebuild draws every stored page again with the drawing of now (store.Rebuild): 204, or its mistakes.
func (s *Server) rebuild(w http.ResponseWriter, r *http.Request) {
	if s.readJSON(w, r) == nil {
		return
	}
	drawing, _ := s.now()
	if err := s.cfg.Store.Rebuild(drawing); err != nil {
		refuse(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// stringFields sets each of fields that input sends to its value, empty or not, leaving the rest nil;
// false when one sent is not a string. The store trims them.
func stringFields(input map[string]json.RawMessage, fields map[string]**string) bool {
	for key, field := range fields {
		if raw, ok := input[key]; ok {
			v, isString := decode[string](raw)
			if !isString {
				return false
			}
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
	// No page is not found, whatever is sent.
	if s.cfg.Store.ReadMeta(slug) == nil {
		notFound(w)
		return
	}
	input := s.readJSON(w, r)
	if input == nil {
		return
	}
	var ed store.Edit
	if !stringFields(input, map[string]**string{
		"title": &ed.Title, "description": &ed.Description, "project": &ed.Project, "slug": &ed.Slug,
	}) {
		send(w, http.StatusBadRequest, "send the title, description, project and slug as strings")
		return
	}
	if raw, ok := input["favorite"]; ok {
		v, isBool := decode[bool](raw)
		if !isBool {
			send(w, http.StatusBadRequest, "send favorite as true or false")
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

// remove deletes a page: 404 for none, its mistake for one that cannot be removed. Other sites and the sandboxed
// pages (Origin: null) are refused. It takes no body: the management page sends none, so requiring JSON would
// make deleting impossible.
func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return
	}
	err := s.cfg.Store.Remove(r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
