package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/servetest"
)

// served is a server of floe with a template of its own that reads images, and data files in home/p for it:
// the settings of a command that asks it, and that directory.
func served(t *testing.T) (env map[string]string, dir string) {
	t.Helper()

	s := servetest.Start(t, map[string]string{
		"templates/pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>" +
			"{{range .Data.i}}<img src=\"{{.}}\">{{end}}</body></html>\n",
		"templates/pic/schema.json":  `{"type": "object", "properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`,
		"templates/pic/example.yaml": "lang: en\ntemplate: pic\n",
		"home/p/d.yaml":              "lang: en\ntemplate: pic\ntitle: D\ni: [img/a.png]\n",
		"home/p/img/a.png":           "\x89PNG\r\n\x1a\n0000",
		"home/p/gone.yaml":           "lang: en\ntemplate: pic\ni: [img/none.png]\n",
		"home/p/h.html":              "<title>H</title>",
	})
	return s.Env, filepath.Join(s.Dir, "home", "p")
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
