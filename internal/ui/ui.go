// Package ui embeds the HTML that floe serves around and into the pages.
package ui

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"strings"

	"floe/internal/i18n"
	"floe/internal/store"
)

// Screen is a screen of the management page, as the server picks it by the path.
type Screen string

// The screens: the page list (/), the task list (/tasks) and the frame around a page (/a/{slug}/).
const (
	List  Screen = "list"
	Tasks Screen = "tasks"
	Frame Screen = "frame"
)

//go:embed web/dist/index.html
var reactSource string

// The two marks of web/dist/index.html that reactPage fills for each response.
const (
	langMark = `<html lang="en">`
	bootMark = `<script type="application/json" id="boot"></script>`
)

// reactParts is web/dist/index.html cut at its marks: before the lang, between the lang and the boot, after the boot.
var reactParts = mustCut(reactSource)

// boot is what the server tells the React page as it starts, the JSON of its script #boot.
type boot struct {
	Screen   Screen            `json:"screen"`
	Lang     i18n.Lang         `json:"lang"`
	Messages map[string]string `json:"messages"`
	Framed   string            `json:"framed"`
	// Served tells that the page of a folder framed is there, as the server found it
	Served bool `json:"served"`
	// Version is what the frame's ?v= asks, as the server read it: a version's number, 0 for the latest, gone for no such
	// version; "" where ?v= asks none.
	Version string `json:"version"`
	// States is the states of a task, for the task list to read when tasks.json cannot be
	States []store.State `json:"states"`
	// MaxURLs is the most URLs the store lets a task have, which the edit dialog tells before a write is refused
	MaxURLs int `json:"maxURLs"`
}

// Page renders the management page of screen, which is also the frame around each page, in the language of c: the
// frame around the page framed, a slug, or "" for the lists. served tells that the folder's page it frames is there.
func Page(c *i18n.Catalog, screen Screen, framed string, served bool, version string) ([]byte, error) {
	return reactPage(newBoot(c, screen, framed, served, version))
}

func newBoot(c *i18n.Catalog, screen Screen, framed string, served bool, version string) boot {
	return boot{
		Screen: screen, Lang: c.Lang(), Messages: c.Formats("ui_"), Framed: framed, Served: served, Version: version,
		States: store.States, MaxURLs: store.MaxURLs,
	}
}

// reactPage is web/dist/index.html with its marks filled from b. json.Marshal escapes <, >, &, U+2028 and U+2029, so
// that no text of b closes the script or breaks its line.
func reactPage(b boot) ([]byte, error) {
	j, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("write the boot values: %w", err)
	}
	var out bytes.Buffer
	out.WriteString(reactParts[0])
	out.WriteString(`<html lang="` + template.HTMLEscapeString(string(b.Lang)) + `">`)
	out.WriteString(reactParts[1])
	out.WriteString(`<script type="application/json" id="boot">`)
	out.Write(j)
	out.WriteString(`</script>`)
	out.WriteString(reactParts[2])
	return out.Bytes(), nil
}

var errMarks = errors.New("web/dist/index.html is not the management page's build: run just web-build")

// cutPage cuts src at langMark and bootMark, which it holds once each in that order.
func cutPage(src string) ([3]string, error) {
	var parts [3]string
	for _, mark := range []string{langMark, bootMark} {
		if n := strings.Count(src, mark); n != 1 {
			return parts, fmt.Errorf("%w: it holds %s %d times, not once", errMarks, mark, n)
		}
	}
	before, rest, _ := strings.Cut(src, langMark)
	between, after, ok := strings.Cut(rest, bootMark)
	if !ok {
		return parts, fmt.Errorf("%w: it holds %s before %s", errMarks, bootMark, langMark)
	}
	return [3]string{before, between, after}, nil
}

func mustCut(src string) [3]string {
	parts, err := cutPage(src)
	if err != nil {
		panic(err) //nolint:forbidigo // the embedded build, as the embedded build is broken for every run
	}
	return parts
}

//go:embed sample/ja.yaml
var sampleJa []byte

//go:embed sample/en.yaml
var sampleEn []byte

// Sample is the data file the management page's theme cards are drawn from, in lang (English for one it has none in),
// and the name its mistakes are told under.
func Sample(lang i18n.Lang) (string, []byte) {
	if lang == "ja" {
		return "sample/ja.yaml", bytes.Clone(sampleJa)
	}
	return "sample/en.yaml", bytes.Clone(sampleEn)
}
