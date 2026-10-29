package cli

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/usererr"
)

// A command line floe cannot read is the user's mistake, told in cobra's words, not floe's failure.
func TestUsageIsAMistake(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version", "x"}, {"--nope"}} {
		root, _ := newRoot(Env{Stdout: io.Discard, Stderr: io.Discard, Dir: "", Getenv: nil}, io.Discard)
		root.SetArgs(args)
		err := root.Execute()
		r := usererr.Tell(english, err)
		if r.Failure != "" || len(r.Mistakes) != 1 || err.Error() != r.Render("") {
			t.Errorf("floe %v told %+v, %q, want one mistake", args, r, err)
		}
	}
}

// encode sends a nil map as null, so that the server asks for the images, and tells a text that is no UTF-8 by
// where it was given; every text a publish sends has a place, and one without is named by its field.
func TestEncode(t *testing.T) {
	t.Parallel()

	b, err := encode(server.Upload{File: "d.yaml", Source: []byte("x")}, sentAt) //nolint:exhaustruct // the rest not given
	if err != nil || !strings.Contains(string(b), `"images":null`) {
		t.Errorf("encoded %s, %v", b, err)
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[server.Link](), reflect.TypeFor[server.Upload]()} {
		for field := range typ.Fields() {
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if t := field.Type; t.Kind() == reflect.Pointer {
				field.Type = t.Elem()
			}
			if field.Type.Kind() == reflect.String && sentAt[name] == "" {
				t.Errorf("%s.%s: no place in sentAt", typ.Name(), name)
			}
		}
	}
	// v2 still tells bytes no UTF-8 as notUTF8 matches them.
	if _, err := jsonv2.Marshal("a\xffb"); !notUTF8(err) {
		t.Errorf("encoding/json/v2 tells bytes no UTF-8 by %v, which notUTF8 does not match", err)
	}
	// Another syntax encoding/json/v2 tells is floe's failure: the body floe made is wrong.
	if _, err := encode(jsontext.Value(`{"a":1,"a":2}`), sentAt); err == nil || usererr.Tell(english, err).Failure == "" {
		t.Errorf("a key twice told %v, want floe's failure", err)
	}
	bad := "a\xffb"
	for body, at := range map[any]string{
		server.Link{Path: "/x", Title: &bad}: "--title", //nolint:exhaustruct // the rest not given
		struct {
			X string `json:"x"`
		}{bad}: `the request's "x"`,
	} {
		_, err := encode(body, sentAt)
		if got := usererr.Tell(english, err).Render(""); got != at+" is not UTF-8; the server takes only UTF-8: rename it, or give it in UTF-8" {
			t.Errorf("%+v told %q", body, got)
		}
	}
}
