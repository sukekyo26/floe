package yamlpos

// Decode returns the value of n, a node Parse returned: keys and values as map[string]any, items as []any, and a
// scalar as YAML 1.2's core schema reads it (core): a plain one as null (nil), true or false, a whole number (int,
// or uint64 past int64), a float64, or else text; one in quotes or a | or > text as text. go.yaml.in/yaml/v4's
// tags, which keep readings of YAML 1.1 (017 as octal, a date), are not taken.
func Decode(n *Node) any {
	switch n.Kind {
	case Mapping:
		m := make(map[string]any, len(n.Pairs))
		for _, p := range n.Pairs {
			m[p.Name()] = Decode(p.Value)
		}
		return m
	case Sequence:
		items := make([]any, len(n.Items))
		for i, item := range n.Items {
			items[i] = Decode(item)
		}
		return items
	case Scalar:
	}
	if n.Style != Plain {
		return n.Value
	}
	r := read12(n.Value)
	switch r.kind {
	case kindNull:
		return nil
	case kindTrue:
		return true
	case kindFalse:
		return false
	case kindInt:
		if r.i.IsInt64() {
			return int(r.i.Int64())
		}
		return r.i.Uint64() // Parse refused one past uint64
	case kindFloat:
		return r.f
	case kindText:
	}
	return n.Value
}
