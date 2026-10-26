package kit

import (
	"html/template"
	"strings"
	"testing"
)

// num groups the digits and rounds to significant digits without the float's noise; bytes picks the
// unit; and both refuse what is not a number.
func TestFormat(t *testing.T) {
	t.Parallel()

	tenth, fifth := 0.1, 0.2
	for _, tt := range []struct{ src, want string }{
		{"{{1234567 | num}}", "1,234,567"},
		{"{{.u | num}}", "18,446,744,073,709,551,615"},
		{"{{-1234.5 | num}}", "-1,234.5"},
		{"{{0.0064812 | num 3}}", "0.00648"},
		{"{{1234567 | num 3}}", "1,230,000"},
		{"{{.f | num}}", "0.30000000000000004"},
		{"{{.f | num 2}}", "0.3"},
		{"{{999 | num}} {{100 | num 1}}", "999 100"},
		{"{{512 | bytes}}", "512 B"},
		{"{{1023 | bytes}}", "1,023 B"},
		{"{{1047552 | bytes}}", "1,023 KiB"},
		{"{{1048575 | bytes}}", "1 MiB"},
		{"{{-1536 | bytes}}", "-1.5 KiB"},
		{"{{.big | num}} {{.big | num 20}} {{.big | num 3}}", "9,007,199,254,740,993 9,007,199,254,740,993 9,010,000,000,000,000"},
		{"{{-1250 | num 2}} {{.u | num 3}}", "-1,300 18,400,000,000,000,000,000"},
		{"{{.z | num}} {{.z | num 2}}", "0 0"},
		{"{{1536 | bytes}}", "1.5 KiB"},
		{"{{1048576 | bytes}}", "1 MiB"},
		{"{{123456789 | bytes}}", "118 MiB"},
		{"{{.u | bytes}}", "16 EiB"},
	} {
		page, err := template.New("t").Funcs(funcs).Parse(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		if err := page.Execute(&b, map[string]any{"u": uint64(18446744073709551615), "f": tenth + fifth, "z": -tenth * 0, "big": int64(9007199254740993)}); err != nil || b.String() != tt.want {
			t.Errorf("%s = %q, %v; want %s", tt.src, b.String(), err, tt.want)
		}
	}
	for _, tt := range []struct{ src, want string }{
		{`{{"12" | num}}`, "num: 12 (string) is not a number"},
		{`{{1 | num 0}}`, "num: write num or num with a count"},
		{`{{1 | num 1 2}}`, "num: write num or num with a count"},
		{`{{true | bytes}}`, "bytes: true (bool) is not a number"},
	} {
		page := template.Must(template.New("t").Funcs(funcs).Parse(tt.src))
		if err := page.Execute(&strings.Builder{}, nil); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s = %v, want %s", tt.src, err, tt.want)
		}
	}
}
