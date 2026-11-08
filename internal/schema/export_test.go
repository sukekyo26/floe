package schema

// ShapeFieldsMissing returns those of names shapeFields does not sort.
func ShapeFieldsMissing(names []string) []string {
	var missing []string
	for _, n := range names {
		if _, ok := shapeFields[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing
}
