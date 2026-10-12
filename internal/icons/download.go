package icons

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sukekyo26/floe/internal/usererr"
)

// The limits of reading a package. A download gives up when no response comes in headerTimeout or
// no data for stallTimeout, but a slow one that keeps going is let finish. The AWS package is about
// 14MB and an icon a few KB.
var (
	headerTimeout       = 30 * time.Second
	stallTimeout        = 60 * time.Second
	maxPackage    int64 = 256 << 20
	maxIcon       int64 = 1 << 20
)

var (
	errTooLarge = errors.New("larger than any icon")
	errStalled  = errors.New("the download stalled")
)

// readAtMost reads r to its end, or stops with errTooLarge past max bytes.
func readAtMost(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err == nil && int64(len(b)) > limit {
		return nil, errTooLarge
	}
	return b, err //nolint:wrapcheck // worded by the caller
}

func read(ctx context.Context, source string) ([]byte, error) {
	if IsURL(source) {
		return download(ctx, source)
	}
	f, err := os.Open(source) //nolint:gosec // G304: the zip the user named
	if err != nil {
		return nil, usererr.CannotRead(source, err)
	}
	defer f.Close() //nolint:errcheck // read only
	b, err := readAtMost(f, maxPackage)
	if errors.Is(err, errTooLarge) {
		return nil, usererr.New("icons_too_large", source, maxPackage>>20)
	}
	if err != nil {
		return nil, usererr.CannotRead(source, err)
	}
	return b, nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, usererr.New("icons_download", url, err.Error())
	}
	transport, _ := http.DefaultTransport.(*http.Transport) //nolint:errcheck // the standard one
	transport = transport.Clone()
	transport.ResponseHeaderTimeout = headerTimeout
	// No Timeout over the whole download: the stall timer below stops one that no longer moves.
	client := &http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: 0}
	res, err := client.Do(req)
	if err != nil {
		return nil, usererr.New("icons_download", url, err.Error())
	}
	defer res.Body.Close() //nolint:errcheck // read only
	if res.StatusCode != http.StatusOK {
		return nil, usererr.New("icons_download", url, "HTTP "+strconv.Itoa(res.StatusCode))
	}
	if res.ContentLength > maxPackage {
		return nil, usererr.New("icons_too_large", url, maxPackage>>20)
	}
	stall := time.AfterFunc(stallTimeout, func() { cancel(errStalled) })
	defer stall.Stop()
	b, err := readAtMost(progress{r: res.Body, on: func() { stall.Reset(stallTimeout) }}, maxPackage)
	if errors.Is(err, errTooLarge) {
		return nil, usererr.New("icons_too_large", url, maxPackage>>20)
	}
	if err != nil {
		reason := err.Error()
		if errors.Is(context.Cause(ctx), errStalled) {
			reason = "no data for " + stallTimeout.String()
		}
		return nil, usererr.New("icons_download", url, reason)
	}
	return b, nil
}

// progress calls on whenever data comes.
type progress struct {
	r  io.Reader
	on func()
}

func (p progress) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.on()
	}
	return n, err //nolint:wrapcheck // a reader passes its reader's errors on
}
