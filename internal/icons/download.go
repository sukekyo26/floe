package icons

import (
	"cmp"
	"context"
	"errors"
	"fmt"
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
	errTooLarge   = errors.New("larger than any icon")
	errNoResponse = errors.New("no response")
	errNoData     = errors.New("no data")
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
		return nil, usererr.About(source, "icons_too_large", source, maxPackage>>20)
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
	// Timers give up on the download: after headerTimeout without a response, then after
	// stallTimeout without data. No timeout covers the whole of it, so a slow one is let finish.
	waiting := time.AfterFunc(headerTimeout, func() { cancel(fmt.Errorf("%w in %v", errNoResponse, headerTimeout)) })
	res, err := http.DefaultClient.Do(req)
	waiting.Stop()
	if err != nil {
		return nil, usererr.New("icons_download", url, cmp.Or(context.Cause(ctx), err).Error())
	}
	defer res.Body.Close() //nolint:errcheck // read only
	if res.StatusCode != http.StatusOK {
		return nil, usererr.New("icons_download", url, "HTTP "+strconv.Itoa(res.StatusCode))
	}
	if res.ContentLength > maxPackage {
		return nil, usererr.New("icons_too_large", url, maxPackage>>20)
	}
	stall := time.AfterFunc(stallTimeout, func() { cancel(fmt.Errorf("%w for %v", errNoData, stallTimeout)) })
	defer stall.Stop()
	b, err := readAtMost(progress{r: res.Body, on: func() { stall.Reset(stallTimeout) }}, maxPackage)
	if errors.Is(err, errTooLarge) {
		return nil, usererr.New("icons_too_large", url, maxPackage>>20)
	}
	if err != nil {
		return nil, usererr.New("icons_download", url, cmp.Or(context.Cause(ctx), err).Error())
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
