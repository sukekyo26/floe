package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/dirs"
	floepublish "floe/internal/publish"
	"floe/internal/store"
	"floe/internal/testfs"
)

// The list shows a link's path and a data file's place as people write them, without \\?\, while meta.json keeps
// them as files.Real gives them.
func TestListedShown(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	html := filepath.Join(home, "page.html")
	if err := os.WriteFile(html, []byte("<title>p</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	dw := picKit(t, "", "")
	e, err := floepublish.PrepareLink(t.Context(), s, dw, html, store.Options{}, "", filepath.Dir(home))
	if err == nil {
		err = s.Save(e)
	}
	if err != nil {
		t.Fatal(told(err, root))
	}
	if !strings.HasPrefix(s.ReadMeta(e.Slug).Path, `\\?\`) {
		t.Fatalf("meta.json path = %q, want it as files.Real gives it", s.ReadMeta(e.Slug).Path)
	}
	place := `\\?\` + filepath.Join(home, "d.yaml")
	up := floepublish.Upload{File: "d.yaml", Source: []byte(ownSrc), Images: map[string][]byte{}, Project: "p", Repo: "", Place: place}
	d, err := floepublish.PrepareData(t.Context(), s, dw, up, store.Options{})
	if err == nil {
		err = s.Save(d)
	}
	if err != nil {
		t.Fatal(told(err, root))
	}
	list, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range list {
		if strings.Contains(l.Path, `\\?\`) || strings.Contains(l.PublishedFile, `\\?\`) {
			t.Errorf("listed %s with %q, %q, want them without \\\\?\\", l.Slug, l.Path, l.PublishedFile)
		}
	}
	if got := s.ReadMeta(d.Slug).PublishedFile; got != place {
		t.Errorf("meta.json publishedFile = %q, want %q kept", got, place)
	}
}

// A link given relative to a drive's own working directory is refused for it, even with no working directory.
func TestLinkDriveRelative(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	_, err := floepublish.PrepareLink(t.Context(), s, picKit(t, "", ""), "C:b.html", store.Options{}, "", filepath.Dir(root))
	if keyOf(err) != "path_drive_relative" {
		t.Errorf("link C:b.html refused %v, want path_drive_relative", err)
	}
}

// A folder this user may not list is not linked: its real path cannot be told (the system denies opening it).
func TestLinkDeniedFolder(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	folder := filepath.Join(home, "denied")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	testfs.DenyList(t, dirs.Shown(folder)) // icacls takes the path as people write it
	_, err := floepublish.PrepareLink(t.Context(), s, picKit(t, "", ""), folder, store.Options{}, "", filepath.Dir(home))
	if keyOf(err) != "library" {
		t.Errorf("link of a denied folder = %v, want it refused in the system's words (CannotRead)", err)
	}
}

// A file on a share is refused before it is opened, as written with \\host\share and through a network drive
// (path_not_local, which files tells: a share is not opened to look at it), but not through a subst drive of a local
// directory.
func TestPrepareLinkShare(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.html"), []byte("<title>a</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	dw := picKit(t, "", "")
	shared, mapped := testfs.MapShare(t, tmp)
	for _, p := range []string{shared, mapped, testfs.SubstShare(t, tmp) + `\`} {
		file := filepath.Join(p, "a.html")
		if _, err := floepublish.PrepareLink(t.Context(), s, dw, file, store.Options{}, home, home); keyOf(err) != "path_not_local" {
			t.Errorf("link %s = %v, want path_not_local", file, err)
		}
	}
	drive := testfs.Subst(t, tmp)
	file := drive + `\a.html`
	if _, err := floepublish.PrepareLink(t.Context(), s, dw, file, store.Options{}, home, home); err != nil {
		t.Errorf("link %s = %v, want it linked", file, err)
	}
}
