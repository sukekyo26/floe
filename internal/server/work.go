package server

import (
	"errors"
	"net/http"

	"floe/internal/store"
)

// tasks answers the tasks.
func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.cfg.Store.ReadTasks()
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, tasks)
}

// editWork writes pages and their tasks in one write (store.Work), and answers each task and page as written. What only
// a sender's mistake makes, which floe's own never send, is refused as such.
func (s *Server) editWork(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r, maxBody)
	if !ok {
		return
	}
	in := decodeStrict[store.WorkWrite](w, body)
	if in == nil {
		return
	}
	if len(in.Slugs) == 0 && (in.Category != nil || in.Pin != nil || in.Clear) {
		send(w, http.StatusBadRequest, "category, pin and clear go with slugs")
		return
	}
	if len(in.Slugs) > 0 && in.TaskProject != nil {
		send(w, http.StatusBadRequest, "taskProject goes without slugs: a page's task is of the page's project")
		return
	}
	written, err := s.cfg.Store.Work(*in)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	sendJSON(w, http.StatusOK, written)
}

// forgetTask removes the task the query names: not found for none, refused while a page or a child is on it.
// LIMIT: the query is read for taskProject and task alone, and a key it does not know is not refused, so a project
// beside taskProject is left unread. floe's own CLI and screen send taskProject only; count the keys if a client
// comes to send both.
func (s *Server) forgetTask(w http.ResponseWriter, r *http.Request) {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return
	}
	q := r.URL.Query()
	err := s.cfg.Store.ForgetTask(q.Get("taskProject"), q.Get("task"))
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
