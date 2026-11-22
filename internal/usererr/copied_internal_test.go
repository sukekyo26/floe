package usererr

import (
	"reflect"
	"testing"
)

// A mistake copied comes back as it was: its key, its arguments of each kind, its place and the mark of one of the
// whole output; an argument of another kind is refused, not worded otherwise.
func TestCopy(t *testing.T) {
	t.Parallel()

	for _, e := range []*Error{
		NewAt("a.yaml", 3, 4, "k", "text", 7, uint8(2), 1.5),
		NewIn("s.json", "#/a", "k"),
		NewWhole("k", "x"),
		New("k"),
	} {
		c, err := Copy(e)
		if err != nil {
			t.Fatalf("Copy(%v): %v", e, err)
		}
		got := c.Error()
		want := *e
		want.Args = make([]any, len(e.Args))
		for i, a := range e.Args {
			switch v := reflect.ValueOf(a); {
			case v.CanInt():
				want.Args[i] = v.Int()
			case v.CanUint():
				want.Args[i] = int64(v.Uint()) //nolint:gosec // G115: small
			default:
				want.Args[i] = a
			}
		}
		if !reflect.DeepEqual(*got, want) {
			t.Errorf("copied %+v, want %+v", *got, want)
		}
	}
	if _, err := Copy(New("k", struct{}{})); err == nil {
		t.Error("an argument of another kind was copied")
	}
}
