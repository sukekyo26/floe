package diagram

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"
)

// Every rect and path a layout holds, in every list of them, carries the index of the part written it comes of, so
// that a list added without stamp fails here.
func TestStampedEveryList(t *testing.T) {
	t.Parallel()

	all, err := Examples()
	if err != nil {
		t.Fatal(err)
	}
	rects, paths := reflect.TypeFor[[]rect](), reflect.TypeFor[[]path]()
	for _, e := range all {
		s, err := read(e.value)
		if err != nil {
			t.Fatal(err)
		}
		lay := newLayout(s, Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
		l := reflect.ValueOf(lay).Elem()
		for i := range l.NumField() {
			f := l.Field(i)
			if f.Type() != rects && f.Type() != paths {
				continue
			}
			if f.Type() == rects && !slices.ContainsFunc(lay.placedRects(), func(rs *[]rect) bool {
				return f.Addr().UnsafePointer() == unsafe.Pointer(rs) //nolint:gosec // comparing addresses only
			}) {
				t.Errorf("%s is not among placedRects, so stamp leaves it", l.Type().Field(i).Name)
			}
			if f.Type() == paths && f.Addr().UnsafePointer() != unsafe.Pointer(&lay.paths) { //nolint:gosec // addresses only
				t.Errorf("%s is no list stamp sees", l.Type().Field(i).Name)
			}
			for j := range f.Len() {
				el := f.Index(j)
				at, part := el.FieldByName("at").String(), int(el.FieldByName("part").Int())
				if p := pointer(part); at != p && !strings.HasPrefix(at, p+"/") {
					t.Errorf("%s: %s[%d] at %s carries part %d", e.Name, l.Type().Field(i).Name, j, at, part)
				}
			}
		}
	}
}
