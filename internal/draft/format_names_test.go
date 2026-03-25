package draft

import "testing"

func TestFormatNames(t *testing.T) {
	cases := []struct {
		names []string
		want  string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"Alice"}, "Alice"},
		{[]string{"Alice", "Bob"}, "Alice and Bob"},
		{[]string{"Alice", "Bob", "Carol"}, "Alice, Bob, and Carol"},
		{[]string{"Alice", "Bob", "Carol", "Dave"}, "Alice, Bob, Carol, and Dave"},
	}
	for _, tc := range cases {
		got := formatNames(tc.names)
		if got != tc.want {
			t.Errorf("formatNames(%v) = %q, want %q", tc.names, got, tc.want)
		}
	}
}
