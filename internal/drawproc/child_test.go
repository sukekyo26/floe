package drawproc_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"floe/internal/drawhere"
	"floe/internal/drawproc"
	"floe/internal/guard"
	"floe/internal/i18n"
	"floe/internal/kit"
	"floe/internal/usererr"
)

// child makes the test binary floe __draw of build: "draw" draws each job, "hold" first holds 512 MiB for hold.yaml
// past its watch of 64 MiB, "sleep" sleeps a minute for slow.yaml, and "crash" panics for crash.yaml.
const child = "FLOE_DRAWPROC_TEST_CHILD"

const build = "test-build"

// The pages drawn.
const (
	page     = "lang: en\ntemplate: report\ntitle: P\nsections:\n  - heading: h\n    parts:\n      - text: x\n"
	mistaken = "lang: en\ntemplate: reprot\n"
	warned   = "lang: en\ntemplate: report\ntitle: P\nsections:\n  - heading: h\n    parts:\n      - text: |\n" +
		"          ```\n          code\n          ```\n"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(child); mode != "" {
		os.Exit(serve(mode))
	}
	os.Exit(m.Run())
}

// serve is the test binary run as floe __draw in mode.
func serve(mode string) int {
	k, err := kit.Floe()
	if err != nil {
		return 1
	}
	if mode == "hold" {
		guard.Start(64<<20, func(string) { os.Exit(drawproc.Stopped) }).Drawing("")
	}
	var kept [][]byte
	err = drawproc.Serve(os.Stdin, os.Stdout, build, func(job drawproc.Job) drawproc.Result {
		switch {
		case mode == "hold" && job.File == "hold.yaml":
			// 512 MiB at most, so that a watch that fails ends.
			for range 512 {
				kept = append(kept, bytes.Repeat([]byte{1}, 1<<20))
				time.Sleep(2 * time.Millisecond)
			}
		case mode == "sleep" && job.File == "slow.yaml":
			time.Sleep(time.Minute)
		case mode == "crash" && job.File == "crash.yaml":
			panic("broken")
		}
		return drawhere.One(k, nil, job)
	})
	_ = kept
	if err != nil {
		return 1
	}
	return 0
}

// children is a Child of build that runs this test binary as floe __draw in mode, for 2 seconds a page.
func children(t *testing.T, build, mode string) *drawproc.Child {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return drawproc.NewChild(build, 2*time.Second, func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, self)
		cmd.Env = []string{child + "=" + mode, "GOMEMLIMIT=2GiB", "PATH=" + os.Getenv("PATH")}
		return cmd
	})
}

func job(file, src string) drawproc.Job {
	return drawproc.Job{
		File: file, Source: []byte(src), Theme: "", Default: "", Untitled: "u", CutOff: false, ImagesRoot: "",
		Sent: nil, ImagesDir: "",
	}
}

// told is r as told in English: its mistakes or failure, and its warnings.
func told(r drawproc.Result) usererr.Report {
	report := usererr.Report{Mistakes: nil, Failure: "", Warnings: nil}
	if r.Err != nil {
		report = usererr.Tell(i18n.EnglishCatalog(), r.Err)
	}
	report.Warnings = append(report.Warnings, usererr.Warn(i18n.EnglishCatalog(), r.Warnings)...)
	return report
}

// mistake is the key and file of r's one mistake, "" for none.
func mistake(r drawproc.Result) (string, string) {
	var l usererr.List
	if l.Add(r.Err) != nil || len(l) != 1 {
		return "", ""
	}
	return l[0].Key, l[0].File()
}

// A page floe __draw draws is the page drawn in process, with its mistakes and warnings told alike.
func TestChild(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	s := children(t, build, "draw").Start(t.Context())
	defer s.Close()
	began := time.Now()
	s.Draw(job("p.yaml", page))
	// Not held to a time: the logs of each system's CI tell how long starting a child takes there.
	t.Logf("a child started and drew a page in %v", time.Since(began))
	for _, src := range []string{page, mistaken, warned} {
		got, want := s.Draw(job("p.yaml", src)), drawhere.One(k, nil, job("p.yaml", src))
		if !bytes.Equal(got.Page, want.Page) || !slices.EqualFunc(told(got).Mistakes, told(want).Mistakes, equal) ||
			told(got).Failure != told(want).Failure || !slices.EqualFunc(told(got).Warnings, told(want).Warnings, equal) {
			t.Errorf("drawn of %q = %+v, want %+v", src, told(got), told(want))
		}
	}
}

func equal(a, b usererr.Told) bool { return a == b }

// A child that holds more than its watch lets is ended, its page refused at its file as draw_memory, and the session
// draws on in a new one; so is one past the session's time, as draw_time, and one that fails, as floe's failure.
func TestChildStops(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		mode, file, key string
	}{
		{"hold", "hold.yaml", "draw_memory"},
		{"sleep", "slow.yaml", "draw_time"},
		{"crash", "crash.yaml", ""},
	} {
		s := children(t, build, c.mode).Start(t.Context())
		first := s.Draw(job(c.file, page))
		next := s.Draw(job("p.yaml", page))
		s.Close()
		key, file := mistake(first)
		if c.key == "" && (first.Err == nil || key != "" || !strings.Contains(first.Err.Error(), "broken")) {
			t.Errorf("%s: drawn with %v, want floe's failure telling the panic", c.mode, first.Err)
		}
		if c.key != "" && (key != c.key || file != c.file) {
			t.Errorf("%s: drawn with %v, want %s at %s", c.mode, first.Err, c.key, c.file)
		}
		if next.Err != nil || len(next.Page) == 0 {
			t.Errorf("%s: the next page drawn with %v", c.mode, next.Err)
		}
	}
}

// A child of another build refuses to draw, and so does a session whose context ended; Children sessions at once.
func TestChildRefuses(t *testing.T) {
	t.Parallel()

	s := children(t, "another", "draw").Start(t.Context())
	if key, _ := mistake(s.Draw(job("p.yaml", page))); key != "draw_build" {
		t.Errorf("another build drew, want draw_build")
	}
	s.Close()

	d := children(t, build, "sleep")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second/2)
	defer cancel()
	if r := d.Start(ctx); r != nil {
		got := r.Draw(job("slow.yaml", page))
		r.Close()
		if key, _ := mistake(got); got.Err == nil || key != "" {
			t.Errorf("a drawing whose context ended = %v, want floe's failure", got.Err)
		}
	}

	held := []drawproc.Session{d.Start(t.Context()), d.Start(t.Context())}
	waiting, stop := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer stop()
	third := d.Start(waiting)
	if got := third.Draw(job("p.yaml", page)); got.Err == nil {
		t.Error("a third session drew while two were open")
	}
	third.Close()
	for _, s := range held {
		s.Close()
	}
	s = d.Start(t.Context())
	defer s.Close()
	if got := s.Draw(job("p.yaml", page)); got.Err != nil {
		t.Errorf("a session after the others closed drew with %v", got.Err)
	}
	if errors.Is(t.Context().Err(), context.Canceled) {
		t.Error("the test's context ended")
	}
}
