package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// serving serves s on a port of its own until ctx is done, or the server is asked to stop; Serve's result comes on
// the channel.
func serving(ctx context.Context, t *testing.T, s *Server) (string, <-chan error) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	s.hosts[addr], s.origins["http://"+addr] = true, true
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	return addr, done
}

// client opens a connection for each request: one the transport dials but does not use stays new to the server,
// which Shutdown waits 5 s for.
var client = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

// post sends body to path on addr as the CLI does.
func post(t *testing.T, addr, path, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+addr+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://"+addr)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	return res
}

// ended waits for Serve's result, for 2 s at most.
func ended(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not end")
	}
}

// A server stops when asked by its own pid; asked by another, or by another origin, it goes on serving.
func TestStop(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	addr, done := serving(t.Context(), t, s)
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"pid": 1}`, http.StatusConflict},
		{`{}`, http.StatusConflict},
		{`{"x": 1}`, http.StatusBadRequest},
		{`{"pid": "` + strconv.Itoa(os.Getpid()) + `"}`, http.StatusBadRequest},
	} {
		if res := post(t, addr, "/api/stop", c.body); res.StatusCode != c.want {
			t.Errorf("stop %s = %d, want %d", c.body, res.StatusCode, c.want)
		}
	}
	for _, origin := range []string{"null", "http://example.com"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+addr+"/api/stop",
			strings.NewReader(`{"pid": `+strconv.Itoa(os.Getpid())+`}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("stop from %s = %d, want 403", origin, res.StatusCode)
		}
	}
	select {
	case err := <-done:
		t.Fatalf("Serve ended unasked: %v", err)
	default:
	}
	if res := post(t, addr, "/api/stop", `{"pid": `+strconv.Itoa(os.Getpid())+`}`); res.StatusCode != http.StatusNoContent {
		t.Errorf("stop of its own pid = %d, want 204", res.StatusCode)
	}
	ended(t, done)
}

// A link under way when the server is stopped, asked to or by its context (SIGTERM), is answered, and named by its
// git remote as ever: the git the link runs is not cut off with the context.
func TestStopFinishesRequests(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, how := range []string{"asked", "context"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()

			s, dir := watched(t, false)
			repo := filepath.Join(filepath.Dir(dir), "proj")
			data := filepath.Join(repo, "l.yaml")
			if err := os.MkdirAll(repo, 0o700); err != nil {
				t.Fatal(err)
			}
			write(t, data, mineData)
			for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.com/x/acme.git"}} {
				cmd := exec.CommandContext(t.Context(), "git", args...)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v %s", args, err, out)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			addr, done := serving(ctx, t, s)

			body := `{"path": "` + data + `"}`
			var d net.Dialer
			conn, err := d.DialContext(t.Context(), "tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			head := "POST /api/artifacts HTTP/1.1\r\nHost: " + addr + "\r\nOrigin: http://" + addr +
				"\r\nContent-Type: application/json\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
			if _, err = io.WriteString(conn, head+body[:5]); err != nil {
				t.Fatal(err)
			}
			// LIMIT: the server is given 100 ms to take the request up; nothing tells it has.
			time.Sleep(100 * time.Millisecond)
			if how == "asked" {
				if res := post(t, addr, "/api/stop", `{"pid": `+strconv.Itoa(os.Getpid())+`}`); res.StatusCode != http.StatusNoContent {
					t.Fatalf("stop = %d", res.StatusCode)
				}
			} else {
				cancel()
			}
			if _, err = io.WriteString(conn, body[5:]); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("link = %d %s", res.StatusCode, b)
			}
			ended(t, done)
			if m := s.cfg.Store.ReadMeta("l"); m == nil || m.Project != "acme" {
				t.Errorf("the link's meta = %+v, want the project acme", m)
			}
		})
	}
}

// An event stream open when the server is asked to stop is closed, so that stopping does not wait for it.
func TestStopEndsStreams(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	addr, done := serving(t.Context(), t, s)
	since := s.version("x", s.cfg.Store.ReadMeta("x"), "")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/api/events/x?since="+since, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if _, err = bufio.NewReader(res.Body).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	if res := post(t, addr, "/api/stop", `{"pid": `+strconv.Itoa(os.Getpid())+`}`); res.StatusCode != http.StatusNoContent {
		t.Fatalf("stop = %d", res.StatusCode)
	}
	ended(t, done)
}
