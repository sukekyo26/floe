package kit

import (
	"reflect"
	"slices"
	"testing"

	"floe/internal/i18n"
	"floe/internal/usererr"
)

// told is err as Tell renders it, a mistake a line.
func told(err error) string {
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("")
}

// The functions bound to a part's value are those whose names part.html is read with, each of the type the name
// holds: a part.html parsed with one is drawn with the other.
func TestBoundFuncsMatchNames(t *testing.T) {
	t.Parallel()

	bound := BoundFuncs(nil)
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range boundFuncs {
			if !yield(name) {
				return
			}
		}
	})
	if got := slices.Sorted(func(yield func(string) bool) {
		for name := range bound {
			if !yield(name) {
				return
			}
		}
	}); !slices.Equal(got, names) {
		t.Fatalf("bound %v, named %v", got, names)
	}
	for _, name := range names {
		if b, n := reflect.TypeOf(bound[name]), reflect.TypeOf(boundFuncs[name]); b != n || b.NumIn() != 0 {
			t.Errorf("%s: bound %v, named %v; want the same, of no argument", name, b, n)
		}
		if _, ok := partFuncs[name]; ok {
			t.Errorf("%s is in partFuncs too", name)
		}
	}
}

// A function bound to a part's value tells a mistake in it at its JSON pointer from the root of the value, a key of
// an item not written too (yamlpos.Locate places it at the item); one the value's shape keeps from being read is told
// as kit_part_value at the value or its key.
func TestBoundFuncsPlaces(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, fn string
		value    any
		want     []string
		shape    bool // told as kit_part_value
	}{
		{"bar over", "bars", []any{
			map[string]any{"label": "a", "value": 1},
			map[string]any{"label": "b", "value": 1},
			map[string]any{"label": "c", "value": 9, "of": 4},
		}, []string{"#/2/value"}, false},
		{"bar of", "bars", []any{map[string]any{"label": "a", "value": 1, "of": 0}}, []string{"#/0/of"}, false},
		{"bar unit", "bars", []any{map[string]any{"label": "a", "value": 1, "unit": 3}}, []string{"#/0/unit"}, false},
		{"bar value not written", "bars", []any{map[string]any{"label": "a"}}, []string{"#/0/value"}, false},
		{"bar not an item", "bars", []any{3}, []string{"#/0"}, false},
		{"bars not a list", "bars", map[string]any{"items": []any{}}, []string{"#"}, true},
		{"code start and mark", "code", map[string]any{"text": "a\n", "start": "x", "mark": []any{1, "y"}}, []string{"#/start", "#/mark/1"}, false},
		{"code mark not a list", "code", map[string]any{"text": "a\n", "mark": 2}, []string{"#/mark"}, false},
		{"code text not written", "code", map[string]any{"text": nil}, []string{"#/text"}, true},
		{"code text a number", "code", map[string]any{"text": 3}, []string{"#/text"}, true},
		{"code not keys", "code", "a", []string{"#"}, true},
		{"script end", "script", map[string]any{"js": "</script>", "data": nil}, []string{"#/js"}, false},
		{"script js not written", "script", map[string]any{"data": 1}, []string{"#/js"}, true},
		{"stats change", "stats", []any{map[string]any{"label": "a", "value": "1", "from": "2"}}, []string{"#/0/better"}, false},
		{"stats not an item", "stats", []any{"a"}, []string{"#/0"}, false},
		{"stats not a list", "stats", "a", []string{"#"}, true},
		{"chart series", "chart", map[string]any{"kind": "line", "x": []any{"a"}, "series": []any{
			map[string]any{"name": "s", "values": []any{1, 2}},
		}}, []string{"#/series/0/values"}, false},
	} {
		out := reflect.ValueOf(BoundFuncs(tt.value)[tt.fn]).Call(nil)
		err, isErr := out[1].Interface().(error)
		var all usererr.List
		if !isErr || all.Add(err) != nil || all == nil {
			t.Errorf("%s: %v, want mistakes", tt.name, err)
			continue
		}
		got := make([]string, len(all))
		for i, e := range all {
			got[i] = e.At()
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: at %q, want %q (%s)", tt.name, got, tt.want, told(err))
		}
		if (all[0].Key == "kit_part_value") != tt.shape {
			t.Errorf("%s: told %s", tt.name, all[0].Key)
		}
	}
}
