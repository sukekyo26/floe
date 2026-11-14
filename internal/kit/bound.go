package kit

import (
	"fmt"
	"html/template"

	"floe/internal/usererr"
)

// boundFuncs are the functions of a part.html that read the part's value, each a name alone in partFuncs, which
// draw binds for each part it draws (BoundFuncs): they take no argument, so that the places of their mistakes, at
// a JSON pointer from the root of the part's value, are where they say.
var boundFuncs = template.FuncMap{
	"bars": unbound[[]bar]("bars"), "chart": unbound[*chart]("chart"), "code": unbound[codeText]("code"),
	"script": unbound[template.HTML]("script"), "stats": unbound[[]stat]("stats"),
}

// unbound holds the name of the function name, which draw binds to the value of each part it draws.
func unbound[T any](name string) func() (T, error) {
	return func() (T, error) {
		var none T
		return none, fmt.Errorf("%w: %s: not bound to a part", ErrFunc, name)
	}
}

// BoundFuncs returns the functions of boundFuncs that read value, the value of a part as drawn. A value of a shape
// they do not read is the mistake kit_part_value, at the value or its key: the part's schema, which may let it
// through, says what it takes, and a function's argument no longer has text/template check its type.
func BoundFuncs(value any) template.FuncMap {
	return template.FuncMap{
		"bars": func() ([]bar, error) {
			items, err := listOf("bars", value)
			if err != nil {
				return nil, err
			}
			return barsOf(items)
		},
		"chart": func() (*chart, error) { return chartOf(value) },
		"code": func() (codeText, error) {
			m, text, err := keyedText("code", value, "text")
			if err != nil {
				return codeText{Lines: nil, Before: 0, Digits: 0, Numbered: false}, err
			}
			return codeOf(text, m["start"], m["mark"])
		},
		"script": func() (template.HTML, error) {
			m, js, err := keyedText("script", value, "js")
			if err != nil {
				return "", err
			}
			return scriptOf(js, m["data"])
		},
		"stats": func() ([]stat, error) {
			items, err := listOf("stats", value)
			if err != nil {
				return nil, err
			}
			return statsOf(items)
		},
	}
}

// listOf is value, a part's, as the list the function fn reads.
func listOf(fn string, value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, usererr.New("kit_part_value", fn, ".", "array").PlacedIn("#")
	}
	return items, nil
}

// keyedText is value, a part's, as the keys and values the function fn reads, with the text of its key.
func keyedText(fn string, value any, key string) (map[string]any, string, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, "", usererr.New("kit_part_value", fn, ".", "object").PlacedIn("#")
	}
	text, ok := m[key].(string)
	if !ok {
		return nil, "", usererr.New("kit_part_value", fn, "."+key, "string").PlacedIn("#/" + key)
	}
	return m, text, nil
}
