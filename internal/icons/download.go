package icons

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/usererr"
)

// The limits of fetching a set. A download gives up when no response comes in headerTimeout or no data for
// stallTimeout, but a slow one that keeps going is let finish. The AWS package is about 14MB, holds about 800
// icons, and an icon is a few KB: an entry that inflates far beyond those, or many of them, would fill the disk.
var (
	headerTimeout       = 30 * time.Second
	stallTimeout        = 60 * time.Second
	maxPackage    int64 = 256 << 20 // a download, and the icons written of a package
	maxIcon       int64 = 1 << 20
	maxIcons            = 10000
)

var (
	errTooLarge   = errors.New("over the limit")
	errNoResponse = errors.New("no response")
	errNoData     = errors.New("no data")
)

// readAtMost reads r to its end, or stops with errTooLarge past limit bytes.
func readAtMost(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err == nil && int64(len(b)) > limit {
		return nil, errTooLarge
	}
	return b, err //nolint:wrapcheck // worded by the caller
}

// packageURL is the package that the set's page links: the first link whose URL is one of the set's packages.
func (k *kit) packageURL(ctx context.Context) (string, error) {
	page, err := download(ctx, k.page, "icons_page_download")
	if err != nil {
		return "", err
	}
	base, err := url.Parse(k.page)
	if err != nil {
		return "", fmt.Errorf("read the page of %s: %w", k.name, err)
	}
	z := html.NewTokenizer(bytes.NewReader(page))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return "", usererr.NewAt(k.page, 0, 0, "icons_no_package_link", k.name)
		case html.StartTagToken, html.SelfClosingTagToken:
			if at := k.linkOf(z, base); at != "" {
				return at, nil
			}
		default:
		}
	}
}

// linkOf returns the URL of the tag z is at, an <a> whose href is one of the set's packages, resolved against
// base; "" for any other.
func (k *kit) linkOf(z *html.Tokenizer, base *url.URL) string {
	name, more := z.TagName()
	if string(name) != "a" {
		return ""
	}
	for more {
		var key, value []byte
		key, value, more = z.TagAttr()
		if string(key) != "href" {
			continue
		}
		ref, err := url.Parse(string(value))
		if err != nil {
			return ""
		}
		if at := base.ResolveReference(ref).String(); k.link.MatchString(at) {
			return at
		}
	}
	return ""
}

// download reads what at, a URL, holds; failing tells it with the catalog message key failing, which says what
// to do instead.
func download(ctx context.Context, at, failing string) ([]byte, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, at, nil)
	if err != nil {
		return nil, libraryAt(at, 0, 0, "icons_bad_url", unwrapped(err))
	}
	// Timers give up on the download: after headerTimeout without a response (connecting included), then
	// after stallTimeout without data. No timeout covers the whole of it, so a slow one is let finish.
	waiting := time.AfterFunc(headerTimeout, func() { cancel(fmt.Errorf("%w in %v", errNoResponse, headerTimeout)) })
	res, err := http.DefaultClient.Do(req)
	waiting.Stop()
	if err != nil {
		return nil, libraryAt(at, 0, 0, failing, unwrapped(cmp.Or(context.Cause(ctx), err)))
	}
	defer res.Body.Close() //nolint:errcheck // read only
	if res.StatusCode != http.StatusOK {
		return nil, usererr.NewAt(at, 0, 0, failing, "HTTP "+strconv.Itoa(res.StatusCode))
	}
	if res.ContentLength > maxPackage {
		return nil, usererr.NewAt(at, 0, 0, "icons_download_too_large", maxPackage>>20)
	}
	stall := time.AfterFunc(stallTimeout, func() { cancel(fmt.Errorf("%w for %v", errNoData, stallTimeout)) })
	defer stall.Stop()
	b, err := readAtMost(progress{r: res.Body, on: func() { stall.Reset(stallTimeout) }}, maxPackage)
	if errors.Is(err, errTooLarge) {
		return nil, usererr.NewAt(at, 0, 0, "icons_download_too_large", maxPackage>>20)
	}
	if err != nil {
		return nil, libraryAt(at, 0, 0, failing, unwrapped(cmp.Or(context.Cause(ctx), err)))
	}
	return b, nil
}

// unwrapped is err without the URL net/http puts before its words (*url.Error), as the mistake is at the URL.
func unwrapped(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
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
