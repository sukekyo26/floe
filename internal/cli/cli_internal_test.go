package cli

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"floe/internal/server"
	"floe/internal/store"
	"floe/internal/usererr"
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
	// Every text a command line gives, a string or strings, by its JSON pointer: the bodies of publish and link, the
	// tags of floe tag, and the task's and the page's of floe task, down through their JSON names. These never reach
	// encode with a text no UTF-8, each for its reason.
	unreached := map[string]string{
		"/parentTitle": "the management page alone sends it",
		"/urls/order":  "the management page alone sends it",
	}
	var texts func(typ reflect.Type, at string)
	texts = func(typ reflect.Type, at string) {
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		switch {
		case typ.Kind() == reflect.String || typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.String:
			if sentAt[at] == "" && unreached[at] == "" {
				t.Errorf("%s: no place in sentAt", at)
			}
		case typ.Kind() == reflect.Struct:
			for field := range typ.Fields() {
				if field.Anonymous {
					// embedded: its fields are the struct's own, as encoding/json/v2 reads them
					texts(field.Type, at)
					continue
				}
				name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				texts(field.Type, at+"/"+name)
			}
		}
	}
	texts(reflect.TypeFor[server.Link](), "")
	texts(reflect.TypeFor[server.Upload](), "")
	tags, _ := reflect.TypeFor[server.Edit]().FieldByName("Tags")
	name, _, _ := strings.Cut(tags.Tag.Get("json"), ",")
	texts(tags.Type, "/"+name)
	texts(reflect.TypeFor[store.WorkWrite](), "")
	// v2 still tells bytes no UTF-8 as notUTF8 matches them.
	if _, err := jsonv2.Marshal("a\xffb"); !notUTF8(err) {
		t.Errorf("encoding/json/v2 tells bytes no UTF-8 by %v, which notUTF8 does not match", err)
	}
	// Another syntax encoding/json/v2 tells is floe's failure: the body floe made is wrong.
	if _, err := encode(jsontext.Value(`{"a":1,"a":2}`), sentAt); err == nil || usererr.Tell(english, err).Failure == "" {
		t.Errorf("a key twice told %v, want floe's failure", err)
	}
	bad := "a\xffb"
	// Each text is told by the longest pointer of sentAt that holds it, else by its field at the top.
	for _, c := range []struct {
		body any
		at   string
	}{
		{server.Link{Path: "/x", Title: &bad}, "--title"},                                              //nolint:exhaustruct // the rest not given
		{server.Edit{Tags: &store.TagsEdit{Add: []string{"a", bad}}}, "--add"},                         //nolint:exhaustruct // the rest not given
		{server.Edit{Tags: &store.TagsEdit{Remove: []string{bad}}}, "--remove"},                        //nolint:exhaustruct // the rest not given
		{server.Upload{File: "d.yaml", Images: map[string][]byte{bad: nil}}, `the request's "images"`}, //nolint:exhaustruct // the rest not given
		{struct {
			X string `json:"x"`
		}{bad}, `the request's "x"`},
	} {
		body, at := c.body, c.at
		_, err := encode(body, sentAt)
		if got := usererr.Tell(english, err).Render(""); got != at+" is not UTF-8; the server takes only UTF-8: rename it, or give it in UTF-8" {
			t.Errorf("%+v told %q", body, got)
		}
	}
}

// readable reads the flags on commands of its own, so that the command that runs reads them once: one that gathers
// holds each value given once.
func TestFlagsReadOnce(t *testing.T) {
	t.Parallel()

	env := Env{Stdout: io.Discard, Stderr: io.Discard, Dir: "/", Getenv: func(k string) string {
		return map[string]string{"HOME": "/h", "FLOE_PORT": "0x10"}[k]
	}}
	root, _ := newRoot(env, io.Discard)
	args := []string{"tag", "d", "--add", "a", "--add", "b", "--remove", "c"}
	root.SetArgs(args)
	if err := readable(env, args); err != nil {
		t.Fatal(err)
	}
	if err := root.Execute(); err == nil {
		t.Fatal("floe tag ran without a server")
	}
	cmd, _, err := root.Find(args)
	if err != nil {
		t.Fatal(err)
	}
	add, addErr := cmd.Flags().GetStringArray("add")
	remove, removeErr := cmd.Flags().GetStringArray("remove")
	if addErr != nil || removeErr != nil || !slices.Equal(add, []string{"a", "b"}) || !slices.Equal(remove, []string{"c"}) {
		t.Errorf("--add %q, %v, --remove %q, %v", add, addErr, remove, removeErr)
	}
}
