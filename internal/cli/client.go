package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"floe/internal/daemon"
	"floe/internal/dirs"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/server"
	"floe/internal/usererr"
)

// pagesPath is where the server's API keeps the pages (server.go's routes under /api/pages): GET lists them, and
// /data, /{slug} and what is under it are one page's.
const pagesPath = "/api/pages"

// client calls the API of the server at base, in English as the command line speaks. The server is the
// only one that writes the store: the CLI asks it.
type client struct {
	base string
}

// remoteError is a refusal of the server, told in English: the report it sent, or, for an answer that is not
// a report (a refusal before it reads the request), its text as floe's failure; body is the answer as sent.
type remoteError struct {
	status int
	report usererr.Report
	body   []byte
}

func (e *remoteError) Error() string {
	return e.report.Render("")
}

// Report is the server's report, which usererr.Tell tells as it is.
func (e *remoteError) Report() usererr.Report {
	return e.report
}

// do sends body as JSON (nil for none) and reads a 2xx answer into out (nil to leave it). It sends Origin:
// base, which the server takes as its own, and Accept: JSON for its refusals; one comes back as a *remoteError,
// in JSON for the user's mistakes and floe's failures, or as text for the protocol's (a forbidden origin, a body
// it cannot read), which the client sent wrong.
func (c client) do(ctx context.Context, method, path string, body, out any) error {
	var sent []byte
	if body != nil {
		var err error
		sent, err = encode(body, sentAt)
		if _, mistake := errors.AsType[*usererr.Error](err); mistake {
			return err
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w", method, path, err)
		}
	}
	req, err := daemon.Request(ctx, c.base, method, path, sent)
	if err != nil {
		return err //nolint:wrapcheck // already says what it sent
	}
	// No time limit: the server checks and saves the page before it answers, and a wait given up on
	// would tell a publish failed that the server goes on to save. Ctrl-C stops it.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer res.Body.Close() //nolint:errcheck // read-only
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	if res.StatusCode >= http.StatusMultipleChoices {
		// A refusal without a reason still says what the server answered.
		message := cmp.Or(strings.TrimSpace(string(b)), strconv.Itoa(res.StatusCode)+" "+http.StatusText(res.StatusCode))
		failed := usererr.Report{Mistakes: nil, Failure: message, Warnings: nil}
		e := &remoteError{status: res.StatusCode, report: failed, body: b}
		var told usererr.Report
		if strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") && json.Unmarshal(b, &told) == nil &&
			(len(told.Mistakes) > 0 || told.Failure != "") {
			e.report = told
		}
		return e
	}
	if out == nil {
		return nil
	}
	if err = json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	return nil
}

// readImages reads needed, the images the server asked for, under dir, the directory of the data file named
// file, as the server's drawing reads them (kit.ImageFile), telling every one that cannot be sent at once. A
// mistake of an image is of its name, which the CLI alone can read: it is told at each place the server answered
// the data names it, in file, as floe check tells it, and in the order of the file.
func readImages(dir, file string, needed []server.NeededImage) (map[string][]byte, error) {
	root := files.NewRoot(dir)
	defer root.Close() //nolint:errcheck // only read from
	images := make(map[string][]byte, len(needed))
	var found usererr.List
	for _, n := range needed {
		b, _, err := kit.ImageFile(root, n.Name, dirs.Dir(file), file)
		if failed := found.Add(imagePlaced(err, file, n)); failed != nil {
			return nil, failed //nolint:wrapcheck // already says what failed
		}
		images[n.Name] = b
	}
	if err := found.Sorted().Err(); err != nil {
		return nil, err //nolint:wrapcheck // mistakes for the user as they are
	}
	return images, nil
}

// imagePlaced is err, of reading the image n, a mistake of its name at each place n has in file; err as it is when
// it has a place (a directory that cannot be opened) or is floe's failure.
func imagePlaced(err error, file string, n server.NeededImage) error {
	e, ok := errors.AsType[*usererr.Error](err)
	if !ok || e.File() != "" {
		return err
	}
	if len(n.Places) == 0 {
		// The server answers the names its drawing read, each of an item.
		return fmt.Errorf("the server asked for the image %s with no place in %s", n.Name, file) //nolint:err113 // floe's
	}
	placed := make(usererr.List, len(n.Places))
	for i, p := range n.Places {
		placed[i] = e.PlacedAt(file, p.Line, p.Column)
	}
	return placed
}

// projectGiven is where a project name comes from, of a page (publish) and of a task (floe work).
const projectGiven = "the project name (of the git remote, the repository's directory or the working directory)"

// sentAt names where each field of a body was given, by its JSON pointer: a text in it that is no UTF-8 is told there.
var sentAt = map[string]string{
	"/path": "argument 1", "/file": "argument 1", "/slug": "--slug", "/title": "--title", "/description": "--description",
	"/theme": "--theme", "/tags/add": "--add", "/tags/remove": "--remove",
	"/state": "--state", "/urls/add": "--url", "/urls/remove": "--remove-url", "/categories": "--categories",
	"/parent": "--parent", "/category": "--category", "/task": "--task", "/slugs": "a slug",
	"/project":     projectGiven,
	"/taskProject": projectGiven,
	"/repo":        "the file's repository name (of the git remote or the repository's directory)",
	"/place":       "the file's place (from its repository's root, else its real path)",
}

// notUTF8 reports whether err is encoding/json/v2's for a string's bytes that are no UTF-8, as it tells them now.
// LIMIT: matched with v2's own error, which it does not export; if v2 tells them otherwise, they are floe's failure.
func notUTF8(err error) bool {
	_, probe := jsonv2.Marshal("\xff")
	want, known := errors.AsType[*jsontext.SyntacticError](probe)
	got, syntax := errors.AsType[*jsontext.SyntacticError](err)
	return known && syntax && errors.Is(got.Err, want.Err)
}

// encodeSent is body encoded once, before the server is asked or started, for c.do to send as it is: a text given
// that is no UTF-8 is refused first, by where it was given (sentAt).
func encodeSent(body any) (jsontext.Value, error) {
	b, err := encode(body, sentAt)
	if err != nil {
		return nil, err
	}
	return jsontext.Value(b), nil
}

// encode is body as the server reads it, with encoding/json/v2: nil maps as null (a jsontext.Value as it is). A
// text in it that is no UTF-8 is the user's mistake, named by where it was given (at, the longest pointer that holds
// it), else by its field at the top.
func encode(body any, at map[string]string) ([]byte, error) {
	b, err := jsonv2.Marshal(body, jsonv2.FormatNilMapAsNull(true))
	if syntax, ok := errors.AsType[*jsontext.SyntacticError](err); ok && notUTF8(err) {
		for p := string(syntax.JSONPointer); p != ""; p = p[:strings.LastIndex(p, "/")] {
			if name, ok := at[p]; ok {
				return nil, usererr.New("cli_not_utf8", name)
			}
		}
		field, _, _ := strings.Cut(strings.TrimPrefix(string(syntax.JSONPointer), "/"), "/")
		return nil, usererr.New("cli_not_utf8", fmt.Sprintf("the request's %q", field))
	}
	// Refused before sending, as the server would: it stops reading at its limit, which may break the sending off
	// before its answer comes.
	if len(b) > server.MaxUpload {
		return nil, usererr.New("cli_publish_large", strconv.Itoa(server.MaxUpload>>20)+" MiB")
	}
	return b, err //nolint:wrapcheck // do says what it sent
}
