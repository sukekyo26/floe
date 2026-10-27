// Package servetest starts floe's server for a test, as floe serve runs it: on a free port of 127.0.0.1, with a
// store, a home and directories of one's own in a temporary directory. Only tests import it.
package servetest

import (
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
)

// Site is a server Start started.
type Site struct {
	// Base is the server's URL, http://127.0.0.1:<port>.
	Base string
	// Dir is the temporary directory, its real path: home/, store/, templates/, parts/, themes/ and icons/ are in it.
	Dir string
	// Env are the settings of a command that asks this server, starting none and opening no browser.
	Env map[string]string
}

// Start writes files (paths under the temporary directory, slash-separated) and starts a server there that reads
// templates, parts and themes of its own from templates/, parts/ and themes/. It serves until the test ends.
func Start(t *testing.T, files map[string]string) *Site {
	t.Helper()

	// Real, as link.Check compares the real paths of home and the file linked.
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := func(name string) string { return filepath.Join(tmp, name) }
	// Each made: a publish asks the server for its directories and compares them, and a link needs its home.
	for _, name := range []string{"home", "store", "templates", "parts", "themes", "icons"} {
		if err = os.Mkdir(dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, text := range files {
		at := filepath.Join(tmp, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, p, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	// This build's, so that a command takes the server as its own rather than replacing it.
	build, err := daemon.Build()
	if err != nil {
		t.Fatal(err)
	}
	own := func(name string) kit.Dir { return kit.Dir{FS: os.DirFS(dir(name)), Name: dir(name)} }
	srv := server.New(server.Config{
		Store: store.New(dir("store")), Port: port, Build: build,
		Drawing: func() (store.Drawing, error) {
			floe, err := kit.Floe()
			if err != nil {
				return store.Drawing{Kit: nil, Icons: nil, Err: nil}, err //nolint:wrapcheck // the test's failure
			}
			k, err := kit.Load(floe, kit.Sources{Parts: own("parts"), Themes: own("themes"), Templates: own("templates")})
			return store.Drawing{Kit: k, Icons: nil, Err: nil}, err //nolint:wrapcheck // as above
		},
		Templates: dir("templates"), Parts: dir("parts"), Themes: dir("themes"), Icons: dir("icons"), Home: dir("home"),
	})
	served := make(chan error, 1)
	go func() { served <- srv.Serve(t.Context(), ln) }()
	t.Cleanup(func() {
		if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve: %v", err)
		}
	})
	return &Site{
		Base: "http://127.0.0.1:" + p, Dir: tmp,
		Env: map[string]string{
			"HOME": dir("home"), "FLOE_DIR": dir("store"), "FLOE_PORT": p, "FLOE_TEMPLATES_DIR": dir("templates"),
			"FLOE_PARTS_DIR": dir("parts"), "FLOE_THEMES_DIR": dir("themes"), "FLOE_ICONS_DIR": dir("icons"),
			"BROWSER": "true",
		},
	}
}
