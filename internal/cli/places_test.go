package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/cli"
)

// With no FLOE_DIR, floe publish, run as a process of its own, keeps the page in the default place
// (~/.local/share/floe, under %USERPROFILE% on Windows; its home's environment alone) and starts the server there,
// which stays after the command ends and answers the next one, links a page by the same home, until floe stop stops
// it; part new makes in the default place too.
func TestDefaultPlaces(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	delete(e.vars, "FLOE_DIR")
	home := filepath.Join(e.tmp, "home")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantRun(t, "publish", e.exe(t, self, "publish", "page.yaml"), 0, e.base()+"/a/page/\n", projectLine)
	if _, err := os.Stat(filepath.Join(cli.DefaultPlace(home, "store"), "page", "meta.json")); err != nil {
		t.Errorf("the page is not in the default place: %v", err)
	}
	// The server the publish started answers the next command, run after the publish ended.
	if r := e.exe(t, self, "list"); r.code != 0 || !strings.Contains(r.stdout, "\tpage\t") {
		t.Errorf("list after the publish = %+v", r)
	}
	// A page in a folder of no home is linked: the server's home is the system's (link.HomeKey).
	site := filepath.Join(e.tmp, "site")
	if err := os.Mkdir(site, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "h.html"), []byte("<title>h</title><p>x</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := e.exe(t, self, "publish", "--link", filepath.Join("site", "h.html")); r.code != 0 || !strings.Contains(r.stdout, "/a/h/") {
		t.Errorf("publish --link = %+v", r)
	}
	// Its stderr is not looked at: under coverage, the process says GOCOVERDIR is not set.
	if r := e.exe(t, self, "part", "new", "mine"); r.code != 0 || !strings.HasPrefix(r.stdout, "made "+filepath.Join(cli.DefaultPlace(home, "parts"), "mine")+"\n") {
		t.Errorf("part new = %+v", r)
	}
	if r := e.exe(t, self, "stop"); r.code != 0 || !strings.HasPrefix(r.stdout, "stopped server") {
		t.Errorf("stop = %+v", r)
	}
	if _, up := e.health(t.Context()); up {
		t.Error("the server answers after floe stop")
	}
}
