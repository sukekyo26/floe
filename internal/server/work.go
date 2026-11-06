package server

import (
	"errors"
	"net/http"

	"github.com/sukekyo26/floe/internal/store"
)

// TaskEdit is what PATCH /api/tasks sends: each nil is left as it is.
type TaskEdit struct {
	Title *string   `json:"title,omitzero"`
	State *string   `json:"state,omitzero"`
	Step  *string   `json:"step,omitzero"`
	URL   *string   `json:"url,omitzero"`
	Steps *[]string `json:"steps,omitzero"`
}

// tasks answers the tasks.
func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.cfg.Store.ReadTasks()
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, tasks)
}

// editTask changes the task the query names, as the management page's URLs name it: a name holds any character, and
// no project is "/". It makes one that is not there.
func (s *Server) editTask(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r, maxBody)
	if !ok {
		return
	}
	in := decodeStrict[TaskEdit](w, body)
	if in == nil {
		return
	}
	q := r.URL.Query()
	if err := s.cfg.Store.EditTask(q.Get("project"), q.Get("task"), store.TaskEdit(*in)); err != nil {
		refuse(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// forgetTask removes the task the query names: not found for none, refused while a page is on it.
func (s *Server) forgetTask(w http.ResponseWriter, r *http.Request) {
	if !s.origins[r.Header.Get("Origin")] {
		send(w, http.StatusForbidden, "forbidden origin")
		return
	}
	q := r.URL.Query()
	err := s.cfg.Store.ForgetTask(q.Get("project"), q.Get("task"))
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
