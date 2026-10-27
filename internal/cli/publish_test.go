package cli_test

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
)

// served is a server of floe on a free port, in this process, as floe serve would run it with env: a command
// with env finds it there and asks it, starting none.
func served(t *testing.T) (env map[string]string, dir string) {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
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
	env = map[string]string{
		"HOME": filepath.Join(tmp, "home"), "FLOE_DIR": filepath.Join(tmp, "store"), "FLOE_PORT": p,
		"FLOE_TEMPLATES_DIR": filepath.Join(tmp, "tpl"), "FLOE_PARTS_DIR": filepath.Join(tmp, "parts"),
		"FLOE_THEMES_DIR": filepath.Join(tmp, "themes"), "FLOE_ICONS_DIR": filepath.Join(tmp, "icons"), "BROWSER": "true",
	}
	for name, text := range map[string]string{
		"tpl/pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>" +
			"{{range .Data.i}}<img src=\"{{.}}\">{{end}}</body></html>\n",
		"tpl/pic/schema.json":  `{"type": "object", "properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`,
		"tpl/pic/example.yaml": "lang: en\ntemplate: pic\n",
		"home/p/d.yaml":        "lang: en\ntemplate: pic\ntitle: D\ni: [img/a.png]\n",
		"home/p/img/a.png":     "\x89PNG\r\n\x1a\n0000",
		"home/p/gone.yaml":     "lang: en\ntemplate: pic\ni: [img/none.png]\n",
		"home/p/h.html":        "<title>H</title>",
	} {
		at := filepath.Join(tmp, name)
		if err = os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	build, err := daemon.Build()
	if err != nil {
		t.Fatal(err)
	}
	none := kit.Dir{FS: nil, Name: ""}
	srv := server.New(server.Config{
		Store: store.New(env["FLOE_DIR"]), Port: port, Build: build,
		Drawing: func() (store.Drawing, error) {
			floe, err := kit.Floe()
			if err != nil {
				return store.Drawing{Kit: nil, Icons: nil, Err: nil}, err
			}
			tpl := env["FLOE_TEMPLATES_DIR"]
			k, err := kit.Load(floe, kit.Sources{Parts: none, Themes: none, Templates: kit.Dir{FS: os.DirFS(tpl), Name: tpl}})
			return store.Drawing{Kit: k, Icons: nil, Err: nil}, err
		},
		Templates: env["FLOE_TEMPLATES_DIR"], Parts: env["FLOE_PARTS_DIR"], Themes: env["FLOE_THEMES_DIR"],
		Icons: env["FLOE_ICONS_DIR"], Home: env["HOME"],
	})
	go func() { _ = srv.Serve(t.Context(), ln) }() //nolint:errcheck // ends with the test
	return env, filepath.Join(tmp, "home", "p")
}

// A data file is published through the server, its images sent when it asks for them; listed, drawn again,
// linked, and removed, each told as floe tells its mistakes.
func TestPublishThroughServer(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	url := "http://127.0.0.1:" + env["FLOE_PORT"]
	for _, step := range []struct {
		args      []string
		code      int
		out, errs string
	}{
		{[]string{"publish", "d.yaml"}, 0, url + "/a/d/\n", ""},
		{[]string{"publish", "d.yaml", "--title", "Kept"}, 0, url + "/a/d/\n", `updated "d" (last published `},
		{[]string{"publish", "gone.yaml"}, 1, "", "img/none.png: no such file or directory\n"},
		{[]string{"publish", "--link", "h.html"}, 0, url + "/a/h/\n", ""},
		{[]string{"publish", "--link", "h.html", "--theme", "ink"}, 1, "", "floe: --theme picks the theme of a page drawn from a data file: a linked file is shown as it is\n"},
		{[]string{"list"}, 0, "\th\tH\t", ""},
		{[]string{"rebuild"}, 0, "", ""},
		{[]string{"rm", "d", "none"}, 1, "", "floe: no page \"none\" (see floe list)\n"},
		{[]string{"publish", "x.html"}, 1, "", "floe: x.html is not a data file: publish a page as a data file (.yaml or .yml) for a template such as report, or show HTML as it is with --link\n"},
	} {
		code, out, errs := runIn(dir, env, step.args...)
		if code != step.code || !strings.Contains(out, step.out) || !strings.Contains(errs, step.errs) {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q, %q", step.args, code, out, errs, step.code, step.out, step.errs)
		}
	}
	if code, out, _ := runIn(dir, env, "list"); code != 0 || strings.Contains(out, "\td\t") {
		t.Errorf("floe list after rm = %d, %q", code, out)
	}
}

// Without a server, floe stop says so, and a setting that cannot be read stops every command of the server.
func TestServerCommandsAlone(t *testing.T) {
	t.Parallel()

	free := map[string]string{"HOME": t.TempDir(), "FLOE_PORT": "1"}
	if code, out, _ := runIn("", free, "stop"); code != 0 || out != "server is not running\n" {
		t.Errorf("floe stop = %d, %q", code, out)
	}
	for _, args := range [][]string{{"list"}, {"rm", "x"}, {"rebuild"}, {"serve"}, {"stop"}} {
		code, _, errs := runIn("/", map[string]string{"HOME": "/h", "FLOE_PORT": "0x10"}, args...)
		if code != 1 || errs != "floe: invalid FLOE_PORT \"0x10\": use a port number from 1 to 65535\n" {
			t.Errorf("floe %v = %d, %q", args, code, errs)
		}
	}
}
