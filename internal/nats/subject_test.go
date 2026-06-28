package natsclient

import (
	"errors"
	"testing"
)

func TestSubjectMatchesPattern(t *testing.T) {
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
		got := SubjectMatchesPattern(tc.subject, tc.pattern)
		if got != tc.want {
			t.Fatalf("SubjectMatchesPattern(%q, %q) = %v, want %v", tc.subject, tc.pattern, got, tc.want)
		}
	}
}

func TestResolvePublishSubject(t *testing.T) {
	t.Parallel()

	subject, err := ResolvePublishSubject("", []string{"events.orders"})
	if err != nil || subject != "events.orders" {
		t.Fatalf("single literal subject: %q %v", subject, err)
	}

	_, err = ResolvePublishSubject("", []string{"orders.>"})
	if !errors.Is(err, ErrSubjectRequired) {
		t.Fatalf("expected ErrSubjectRequired, got %v", err)
	}

	subject, err = ResolvePublishSubject("orders.new", []string{"orders.>"})
	if err != nil || subject != "orders.new" {
		t.Fatalf("wildcard match: %q %v", subject, err)
	}
}
