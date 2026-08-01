package subjectauth

import "testing"

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subject string
		pattern string
		want    bool
	}{
		{"orders.new", "orders.>", true},
		{"orders.new", "orders.*", true},
		{"orders.new", "billing.>", false},
		{"foo", "foo", true},
		{"foo.bar", "foo", false},
		{"a.b.c", "a.>", true},
	}
	for _, tc := range tests {
		got := MatchesPattern(tc.subject, tc.pattern)
		if got != tc.want {
			t.Fatalf("MatchesPattern(%q, %q) = %v, want %v", tc.subject, tc.pattern, got, tc.want)
		}
	}
}
