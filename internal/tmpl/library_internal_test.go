package tmpl

import (
	"path/filepath"
	"strings"
	"testing"
)

// A floe fits a template from the version it needs up to the next major, or the next minor under
// major 0.
func TestFits(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		needs, have semver
		want        string
	}{
		{semver{1, 2, 0}, semver{1, 2, 0}, ""},
		{semver{1, 2, 0}, semver{1, 9, 3}, ""},
		{semver{1, 2, 0}, semver{1, 1, 9}, "template t needs floe 1.2.0 or later, and this is floe 1.1.9"},
		{semver{1, 2, 0}, semver{2, 0, 0}, "template t is written for floe 1, not for this floe 2.0.0"},
		{semver{2, 0, 0}, semver{1, 9, 0}, "template t needs floe 2.0.0 or later"},
		{semver{0, 2, 0}, semver{0, 2, 5}, ""},
		{semver{0, 2, 0}, semver{0, 3, 0}, "template t is written for floe 0.2, not for this floe 0.3.0"},
		{semver{0, 2, 1}, semver{0, 2, 0}, "template t needs floe 0.2.1 or later"},
	} {
		err := fits("t", tt.needs, tt.have)
		if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.HasPrefix(err.Error(), tt.want)) {
			t.Errorf("fits(%v, %v) = %v, want %q", tt.needs, tt.have, err, tt.want)
		}
	}
}

// A template gone after it was found is an error, not a template without its optional files.
func TestReadOptionalOfATemplateGone(t *testing.T) {
	t.Parallel()

	gone := &Template{Name: "gone", Dir: filepath.Join(t.TempDir(), "gone"), Path: ""}
	if _, found, err := gone.readOptional("checks.yaml"); err == nil || found {
		t.Errorf("readOptional of a template gone = %v, %v", found, err)
	}
}

// The version is read as written: YAML would read 1.10 as the number 1.1.
func TestNeedsFloe(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src  string
		want semver
	}{
		{"floe: 1.10\n", semver{1, 10, 0}},
		{"floe: '2.3.4'\n", semver{2, 3, 4}},
		{"floe: 1\n", semver{1, 0, 0}},
	} {
		if got, err := needsFloe("template.yaml", []byte(tt.src)); err != nil || got != tt.want {
			t.Errorf("needsFloe(%q) = %v, %v; want %v", tt.src, got, err, tt.want)
		}
	}
	for _, src := range []string{"floe: 1.2.3.4\n", "floe: 01.2\n", "floe: -1\n", "floe: 1.x\n", "floe:\n"} {
		if _, err := needsFloe("template.yaml", []byte(src)); err == nil {
			t.Errorf("needsFloe(%q) took it", src)
		}
	}
}
