package domain

import "testing"

func TestHighestRole(t *testing.T) {
	t.Parallel()
	if got := HighestRole([]string{RoleViewer, RoleOperator}); got != RoleOperator {
		t.Fatalf("got %q want operator", got)
	}
	if got := HighestRole([]string{RoleViewer, RoleAdmin}); got != RoleAdmin {
		t.Fatalf("got %q want admin", got)
	}
	if got := HighestRole(nil); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
