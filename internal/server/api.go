package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

const maxBody = 64 << 10

var jsonTypeRe = regexp.MustCompile(`(?i)^application/json\b`)

// readJSON reads a write from the management page. Other sites and the sandboxed pages are refused by
// the exact Origin and by requiring JSON (which needs a CORS preflight). On refusal it has answered
// and returns nil.
// LIMIT: 127.0.0.1 is open to every user of the machine, and anyone can fake Origin. Do not run floe
// on a shared machine.
func (s *Server) readJSON(w http.ResponseWriter, r *http.Request) map[string]json.RawMessage {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return nil
	}
	if !jsonTypeRe.MatchString(r.Header.Get("Content-Type")) {
		send(w, http.StatusUnsupportedMediaType, "send application/json")
		return nil
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		send(w, http.StatusRequestEntityTooLarge, "request body too large")
		return nil
	}
	if err != nil || !json.Valid(body) {
		send(w, http.StatusBadRequest, catalog(r).Msg("api_not_json"))
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
// else is 500.
func refuse(w http.ResponseWriter, r *http.Request, err error) {
	var ue *usererr.Error
	// The mistakes of a data file, at their lines and columns, are worded too.
	var l i18n.Localizer
	switch {
	case errors.As(err, &ue) && ue.Conflict:
		send(w, http.StatusConflict, ue.Localize(catalog(r)))
	case errors.As(err, &l):
		send(w, http.StatusBadRequest, l.Localize(catalog(r)))
	default:
		send(w, http.StatusInternalServerError, err.Error())
	}
}

type slugBody struct {
	Slug string `json:"slug"`
}

// addLink registers a linked file from the management page. Unlike the CLI there is no --force: a URL
// name another file uses is refused (409) so that a typo never replaces another link.
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
	opts := store.Options{
		Link: true, Slug: text(input["slug"]), Title: nil, Description: text(input["description"]),
		Theme: nil, PartsTheme: nil, Force: false,
	}
	entry, err := s.cfg.Store.Prepare(r.Context(), *path, opts, filepath.Dir(*path))
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

// text is a non-blank string, trimmed; anything else is nil.
func text(raw json.RawMessage) *string {
	var s string
	if json.Unmarshal(raw, &s) != nil || strings.TrimSpace(s) == "" {
		return nil
	}
	s = strings.TrimSpace(s)
	return &s
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
	for key, field := range map[string]**string{
		"title": &ed.Title, "description": &ed.Description, "project": &ed.Project, "slug": &ed.Slug,
	} {
		if raw, ok := input[key]; ok {
			v, isString := decode[string](raw)
			if !isString {
				send(w, http.StatusBadRequest, catalog(r).Msg("api_edit_strings"))
				return
			}
			*field = &v
		}
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
