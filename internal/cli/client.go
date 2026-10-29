package cli

import (
	"bytes"
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

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/usererr"
)

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
	var r io.Reader
	if body != nil {
		b, err := encode(body, sentAt)
		if _, mistake := errors.AsType[*usererr.Error](err); mistake {
			return err
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w", method, path, err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	req.Header.Set("Origin", c.base)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", string(i18n.English))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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

// readImages reads names, the images the server asked for, under dir, the directory of the data file named
// file, as the server's drawing reads them (kit.ImageFile), telling every one that cannot be sent at once.
func readImages(dir, file string, names []string) (map[string][]byte, error) {
	root := files.NewRoot(dir)
	defer root.Close() //nolint:errcheck // only read from
	images := make(map[string][]byte, len(names))
	var found usererr.List
	for _, name := range names {
		b, _, err := kit.ImageFile(root, name, dirs.Dir(file), file)
		if failed := found.Add(err); failed != nil {
			return nil, failed //nolint:wrapcheck // already says what failed
		}
		images[name] = b
	}
	if err := found.Err(); err != nil {
		return nil, err //nolint:wrapcheck // mistakes for the user as they are
	}
	return images, nil
}

// sentAt names where each field of a body was given: a text in it that is no UTF-8 is told there.
var sentAt = map[string]string{
	"path": "argument 1", "file": "argument 1", "slug": "--slug", "title": "--title", "description": "--description",
	"theme":   "--theme",
	"project": "the project name (of the git remote, the repository's directory or the working directory)",
}

// notUTF8 reports whether err is encoding/json/v2's for a string's bytes that are no UTF-8, as it tells them now.
// LIMIT: matched with v2's own error, which it does not export; if v2 tells them otherwise, they are floe's failure.
func notUTF8(err error) bool {
	_, probe := jsonv2.Marshal("\xff")
	want, known := errors.AsType[*jsontext.SyntacticError](probe)
	got, syntax := errors.AsType[*jsontext.SyntacticError](err)
	return known && syntax && errors.Is(got.Err, want.Err)
}

// encode is body as the server reads it, with encoding/json/v2: nil maps as null (a jsontext.Value as it is). A
// text in it that is no UTF-8 is the user's mistake, named by where it was given (at), else by its field.
func encode(body any, at map[string]string) ([]byte, error) {
	b, err := jsonv2.Marshal(body, jsonv2.FormatNilMapAsNull(true))
	if syntax, ok := errors.AsType[*jsontext.SyntacticError](err); ok && notUTF8(err) {
		field := strings.TrimPrefix(string(syntax.JSONPointer), "/")
		field, _, _ = strings.Cut(field, "/")
		return nil, usererr.New("cli_not_utf8", cmp.Or(at[field], fmt.Sprintf("the request's %q", field)))
	}
	// Refused before sending, as the server would: it stops reading at its limit, which may break the sending off
	// before its answer comes.
	if len(b) > server.MaxUpload {
		return nil, usererr.New("cli_publish_large", strconv.Itoa(server.MaxUpload>>20)+" MiB")
	}
	return b, err //nolint:wrapcheck // do says what it sent
}
