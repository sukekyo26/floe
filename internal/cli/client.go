package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// client calls the API of the server at base, in the language lang. The server is the only one that
// writes the store: the CLI asks it.
type client struct {
	base string
	lang i18n.Lang
}

// remoteError is a refusal of the server, in the user's language. mistakes is set for those of a data
// file at their lines and columns, which go out without "floe: "; body is the answer as sent.
type remoteError struct {
	status   int
	message  string
	mistakes bool
	body     []byte
}

func (e *remoteError) Error() string {
	return e.message
}

// do sends body as JSON (nil for none) and reads a 2xx answer into out (nil to leave it). It sends
// Origin: base, which the server takes as its own, and Accept: JSON for its refusals; one comes back as
// a *remoteError, in JSON or, before the server reads the request, as text.
func (c client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
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
	req.Header.Set("Accept-Language", string(c.lang))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	if res.StatusCode >= http.StatusMultipleChoices {
		e := &remoteError{status: res.StatusCode, message: strings.TrimSpace(string(b)), mistakes: false, body: b}
		var refusal server.Refusal
		if strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") && json.Unmarshal(b, &refusal) == nil &&
			refusal.Message != "" {
			e.message, e.mistakes = refusal.Message, refusal.Mistakes
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

// status is the status of the refusal err, 0 for an error of another kind.
func status(err error) int {
	var re *remoteError
	if errors.As(err, &re) {
		return re.status
	}
	return 0
}

// readImages reads names, the images the server asked for, under dir, the data file's directory, as its
// check reads them. One that is not there is not sent, for the server to tell at its line and column.
// LIMIT: an image refused here (a symlink, too large, not a file, unreadable, not holding its type) is
// told without its line and column, which only the server's check knows. Send the server why if that
// comes to matter.
func readImages(dir string, names []string) (map[string][]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, usererr.CannotRead(dir, err)
	}
	defer root.Close()
	images := make(map[string][]byte, len(names))
	for _, name := range names {
		b, err := tmpl.ReadImage(root.FS(), name, dir)
		var ue *usererr.Error
		if errors.As(err, &ue) && ue.Key == "tmpl_image_missing" {
			continue
		}
		if err != nil {
			return nil, err //nolint:wrapcheck // a usererr for the user as is
		}
		images[name] = b
	}
	return images, nil
}
