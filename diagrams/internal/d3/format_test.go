package d3

import "testing"

// Expected values from d3-format 3 (d3.format(spec)(value)).
func TestFormat(t *testing.T) {
	for _, c := range []struct {
		spec string
		v    float64
		want string
	}{
		{",", 1234567.5, "1,234,567.5"},
		{",", 10, "10"},
		{",", 0.35, "0.35"},
		{".1%", 0.35, "35.0%"},
		{",.2f", 1234.5, "1,234.50"},
		{".2s", 42000, "42k"},
		{".3s", 1234567, "1.23M"},
		{"d", 3.7, "4"},
		{".0%", 0.123, "12%"},
		{"+.1f", 2, "+2.0"},
		{".2f", -0.001, "0.00"},
		{",.1f", 15, "15.0"},
		{"e", 1500, "1.500000e+3"},
		{"08.2f", -3.14159, "-0003.14"},
	} {
		f, err := Format(c.spec)
		if err != nil {
			t.Fatal(err)
		}
		if got := f(c.v); got != c.want {
			t.Errorf("format(%q)(%v) = %q, want %q", c.spec, c.v, got, c.want)
		}
	}
}
