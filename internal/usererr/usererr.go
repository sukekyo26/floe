// Package usererr carries a mistake the user can fix, worded for the CLI and for the management page.
package usererr

// Error is a refusal with the English message the CLI prints and the Japanese reason the
// management page shows. Conflict marks a name already in use (HTTP 409 rather than 400).
type Error struct {
	Msg      string
	Ja       string
	Conflict bool
}

// New returns an Error; an empty ja falls back to msg.
func New(msg, ja string) *Error {
	if ja == "" {
		ja = msg
	}
	return &Error{Msg: msg, Ja: ja, Conflict: false}
}

func (e *Error) Error() string {
	return e.Msg
}
