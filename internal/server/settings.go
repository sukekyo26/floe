package server

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/ui"
	"github.com/sukekyo26/floe/internal/usererr"
)

// SettingsPut is a change of settings.json, as the management page's settings send it: the default theme of the pages
// stored from now on.
type SettingsPut struct {
	Theme string `json:"theme"`
}

// Theme is a theme of the drawing of now, as the management page's settings list it: About in the reader's language
// for floe's.
type Theme struct {
	Name  string `json:"name"`
	About string `json:"about"`
	Dir   string `json:"dir"` // its directory when it is one's own, "" for floe's
}

// Themes is what the management page's settings show: the themes, the default theme of settings.json ("" for none
// chosen), and the mistakes that keep the one or the other from being read, as refuse words them.
type Themes struct {
	Themes   []Theme         `json:"themes"`
	Default  string          `json:"default"`
	Drawing  *usererr.Report `json:"drawing,omitzero"`
	Settings *usererr.Report `json:"settings,omitzero"`
}

// putSettings writes the default theme SettingsPut names, a theme of the drawing of now, to settings.json. Pages
// stored already keep the default they were stored with: none is drawn again.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r, maxBody)
	if !ok {
		return
	}
	in := decodeStrict[SettingsPut](w, body)
	if in == nil {
		return
	}
	drawing := s.latest()
	if drawing.Err != nil {
		refuse(w, r, drawing.Err)
		return
	}
	if err := knownTheme(drawing, in.Theme); err != nil {
		refuse(w, r, err)
		return
	}
	if err := s.cfg.Store.SetTheme(in.Theme); err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, SettingsPut{Theme: in.Theme})
}

// knownTheme refuses name when it is no theme of drawing, read without a mistake.
func knownTheme(drawing store.Drawing, name string) error {
	if _, ok := drawing.Kit.Theme(name); ok {
		return nil
	}
	_, _, themes, _ := drawing.Kit.Names()
	return usererr.New("tmpl_unknown_theme", name, strings.Join(themes, ", "))
}

// ThemePick is the theme picked for a page, as the management page sends it: "" takes the pick off.
type ThemePick struct {
	Theme string `json:"theme"`
}

// pickTheme picks the theme ThemePick names, a theme of the drawing of now, for the page slug drawn from a data file,
// drawing the page in it (store.PickTheme): refused as drawn, nothing written, when it cannot be, and 409 when the page
// changed meanwhile.
func (s *Server) pickTheme(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	body, ok := s.readBody(w, r, maxBody)
	if !ok {
		return
	}
	in := decodeStrict[ThemePick](w, body)
	if in == nil {
		return
	}
	drawing := s.latest()
	err := drawing.Err
	if err == nil && in.Theme != "" {
		err = knownTheme(drawing, in.Theme)
	}
	if err == nil {
		err = s.cfg.Store.PickTheme(drawing, slug, in.Theme)
	}
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, ThemePick{Theme: in.Theme})
}

// themes answers Themes. A settings.json that cannot be read still lists the themes, for one to be chosen, which
// writes it whole.
func (s *Server) themes(w http.ResponseWriter, r *http.Request) {
	out := Themes{Themes: []Theme{}, Default: "", Drawing: nil, Settings: nil}
	c := catalog(r)
	if set, err := s.cfg.Store.ReadSettings(); err != nil {
		report := usererr.Tell(c, err)
		out.Settings = &report
	} else {
		out.Default = set.Theme
	}
	drawing := s.latest()
	if drawing.Err != nil {
		report := usererr.Tell(c, drawing.Err)
		out.Drawing = &report
		sendJSON(w, http.StatusOK, out)
		return
	}
	for _, e := range drawing.Kit.Themes() {
		about := e.About
		if e.Dir == "" {
			// floe's own, described in the reader's language; one's own as its theme.yaml has it
			about = c.Msg("theme_about_" + e.Name)
		}
		out.Themes = append(out.Themes, Theme{Name: e.Name, About: about, Dir: e.Dir})
	}
	sendJSON(w, http.StatusOK, out)
}

// themeSample answers the sample page, in the language asked for, drawn in the theme named: headed as a page the
// server draws (servePage), in the sandbox.
// LIMIT: drawn each time it is asked for, once a theme each time the settings open (a few ms each); keep them for the
// drawing they were drawn with if opening comes to be slow.
func (s *Server) themeSample(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	drawing := s.latest()
	if drawing.Err != nil {
		refuse(w, r, drawing.Err)
		return
	}
	_, _, themes, _ := drawing.Kit.Names()
	if !slices.Contains(themes, name) {
		notFound(w)
		return
	}
	file, src := ui.Sample(i18n.FromAcceptLanguage(r.Header.Get("Accept-Language")))
	o := tmpl.Options{
		Theme: name, Default: "", Untitled: "", Images: nil, ImagesDir: "", Icons: drawing.Icons, CutOff: false,
	}
	page, _, err := tmpl.Draw(drawing.Kit, file, src, o)
	if err != nil {
		refuse(w, r, err)
		return
	}
	pageHeaders(w, r, true)
	sendBytes(w, http.StatusOK, htmlType, page)
}

// OwnTheme is the theme a page is drawn in without the theme picked for it, which the management page names on the card
// that removes the pick.
type OwnTheme struct {
	Own string `json:"own"`
}

// ownTheme answers OwnTheme of the page slug drawn from a data file (store.OwnTheme): 404 for a page that takes no
// theme.
func (s *Server) ownTheme(w http.ResponseWriter, r *http.Request) {
	own, err := s.cfg.Store.OwnTheme(r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, OwnTheme{Own: own})
}
