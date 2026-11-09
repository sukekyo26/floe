package server_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
)

const report = "lang: en\ntemplate: report\ntitle: T\nsections:\n  - heading: h\n    parts:\n      - text: x\n"

// inkAccent is in a page drawn in ink and no other.
const inkAccent = "--accent: #2b3a67"

// The default theme is written by name, a theme of the drawing of now; one written draws the pages stored from
// then on, not the ones there.
func TestAPISettings(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	want(t, e.publish(t, "before", report, nil), http.StatusCreated, `"slug":"before"`)
	m := store.New(e.root).ReadMeta("before")
	page := filepath.Join(e.root, "before", m.Source, "page.html")
	was, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	want(t, e.write(t, http.MethodPut, "/api/settings", server.SettingsPut{Theme: "nope"}), http.StatusBadRequest, `there is no theme \"nope\"; use app, aurora,`)
	want(t, e.write(t, http.MethodPut, "/api/settings", server.SettingsPut{Theme: ""}), http.StatusBadRequest, `there is no theme \"\"`)
	unknown := e.do(t, http.MethodPut, "/api/settings", `{"theme": "ink", "x": 1}`, "Origin", origin, "Content-Type", "application/json")
	want(t, unknown, http.StatusBadRequest, `unknown object member name "x"`)
	want(t, e.write(t, http.MethodPut, "/api/settings", server.SettingsPut{Theme: "ink"}), http.StatusOK, `{"theme":"ink"}`)
	if set, err := store.New(e.root).ReadSettings(); err != nil || set.Theme != "ink" {
		t.Errorf("settings.json: %+v, %v", set, err)
	}
	if now, err := os.ReadFile(page); err != nil || string(now) != string(was) {
		t.Errorf("the page there changed: %v", err)
	}
	want(t, e.publish(t, "after", report, nil), http.StatusCreated, `"slug":"after"`)
	m = store.New(e.root).ReadMeta("after")
	if b, err := os.ReadFile(filepath.Join(e.root, "after", m.Source, "page.html")); err != nil || !strings.Contains(string(b), inkAccent) {
		t.Errorf("a page stored after is not in ink: %v", err)
	}
}

// A page's theme is picked by name, a theme of the drawing of now or "" for none, and listed; the page is drawn in it,
// its original theme told apart.
func TestAPIPickTheme(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	want(t, e.publish(t, "d", report, nil), http.StatusCreated, `"slug":"d"`)
	const path = "/api/artifacts/d/theme"
	want(t, e.write(t, http.MethodPut, path, server.ThemePick{Theme: "nope"}), http.StatusBadRequest, `there is no theme \"nope\"; use app, aurora,`)
	unknown := e.do(t, http.MethodPut, path, `{"theme": "ink", "x": 1}`, "Origin", origin, "Content-Type", "application/json")
	want(t, unknown, http.StatusBadRequest, `unknown object member name "x"`)
	want(t, e.write(t, http.MethodPut, "/api/artifacts/none/theme", server.ThemePick{Theme: "ink"}), http.StatusNotFound, "not found")
	want(t, e.write(t, http.MethodPut, path, server.ThemePick{Theme: "ink"}), http.StatusOK, `{"theme":"ink"}`)
	// Its original theme, without the pick
	want(t, e.do(t, http.MethodGet, path, ""), http.StatusOK, `{"own":"default"}`)
	want(t, e.do(t, http.MethodGet, "/api/artifacts/none/theme", ""), http.StatusNotFound, "not found")
	want(t, e.do(t, http.MethodGet, "/api/artifacts", ""), http.StatusOK, `"pickedTheme":"ink"`)
	want(t, e.do(t, http.MethodGet, "/a/d/?raw", ""), http.StatusOK, inkAccent)
	want(t, e.write(t, http.MethodPut, path, server.ThemePick{Theme: ""}), http.StatusOK, `{"theme":""}`)
	if w := e.do(t, http.MethodGet, "/a/d/?raw", ""); strings.Contains(w.Body.String(), inkAccent) {
		t.Error("taken off, still in ink")
	}
}

// The themes listed are those of the drawing, one's own with its directory and its description as written, floe's
// described in the reader's language; a settings.json that cannot be read is told beside them.
func TestAPIThemes(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	var got server.Themes
	read := func(lang string) {
		t.Helper()
		w := e.do(t, http.MethodGet, "/api/themes", "", "Accept-Language", lang)
		if w.Code != http.StatusOK {
			t.Fatalf("answered %d %s", w.Code, w.Body)
		}
		got = server.Themes{}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
	}
	about := func(name string) string {
		for _, th := range got.Themes {
			if th.Name == name {
				return th.About
			}
		}
		return "none"
	}
	read("ja")
	if about("ink") != "紙にインク：黒に近い色、強調には藍色のインク、セクションの区切り線もそのインク" || got.Default != "" || got.Settings != nil {
		t.Errorf("ja: %+v", got)
	}
	read("en")
	if about("ink") != "Ink on paper: near black, an indigo ink for emphasis, its sections ruled in the ink" {
		t.Errorf("en: %+v", got)
	}
	if err := os.MkdirAll(e.root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.root, "settings.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	read("en")
	if got.Settings == nil || len(got.Settings.Mistakes) != 1 || got.Settings.Mistakes[0].File != filepath.Join(e.root, "settings.json") || got.Settings.Mistakes[0].Text != "jsontext: unexpected EOF after offset 1" || about("ink") == "none" {
		t.Errorf("a settings.json broken: %+v", got)
	}
}

// One's own theme is listed with its directory and its theme.yaml's description, in whatever language it is written.
func TestAPIThemesOwn(t *testing.T) {
	t.Parallel()

	floe, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	themes := fstest.MapFS{
		"mine/theme.yaml":  {Data: []byte("description: 私たちの見た目\n")},
		"mine/palette.css": {Data: []byte(":scope { --accent: #123456; }\n")},
	}
	none := kit.Dir{FS: nil, Name: ""}
	k, err := kit.Load(floe, kit.Sources{Parts: none, Themes: kit.Dir{FS: themes, Name: "/t/themes"}, Templates: none})
	if err != nil {
		t.Fatal(err)
	}
	srv := server.New(server.Config{
		Store: store.New(filepath.Join(t.TempDir(), "store")), Port: port, Build: "test", Templates: "", Parts: "", Themes: "/t/themes",
		Icons: "", Home: t.TempDir(), Drawing: func() (store.Drawing, error) { return store.Drawing{Kit: k, Icons: nil, Err: nil}, nil },
	})
	e := env{srv: srv, h: srv.Handler(), root: "", home: "", tpl: ""}
	w := e.do(t, http.MethodGet, "/api/themes", "", "Accept-Language", "en")
	want(t, w, http.StatusOK, `{"name":"mine","about":"私たちの見た目","dir":"/t/themes/mine"}`)
}

// A theme's sample is the sample page drawn in it, headed as a page the server draws, in the sandbox.
func TestAPIThemeSample(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	want(t, e.publish(t, "d", report, nil), http.StatusCreated, `"slug":"d"`)
	page := e.do(t, http.MethodGet, "/a/d/?raw", "")
	w := e.do(t, http.MethodGet, "/api/themes/ink/sample", "", "Accept-Language", "ja")
	want(t, w, http.StatusOK, inkAccent)
	want(t, w, http.StatusOK, "見本のページ")
	if got, of := w.Header().Get("Content-Security-Policy"), page.Header().Get("Content-Security-Policy"); got != of || !strings.Contains(got, "sandbox") || !strings.Contains(got, "connect-src") {
		t.Errorf("CSP %q, a page's %q", got, of)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers %v", w.Header())
	}
	want(t, e.do(t, http.MethodGet, "/api/themes/nope/sample", ""), http.StatusNotFound, "not found")
}

// Each of floe's themes is described on the management page in each language: in English as its theme.yaml has it.
func TestThemeAbouts(t *testing.T) {
	t.Parallel()

	floe, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	en, ja := i18n.New(i18n.English), i18n.New("ja")
	for _, th := range floe.Themes() {
		key := "theme_about_" + th.Name
		if got := en.Msg(key); got != th.About {
			t.Errorf("%s: English %q, theme.yaml %q", th.Name, got, th.About)
		}
		if got := ja.Msg(key); got == key || got == th.About {
			t.Errorf("%s: Japanese %q", th.Name, got)
		}
	}
}
