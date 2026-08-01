package domain

import (
	"testing"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestHighestRole(t *testing.T) {
	t.Parallel()
	if got := HighestRole([]string{RoleViewer, RoleOperator}); got != RoleOperator {
		t.Fatalf("got %q want operator", got)
	}
	if got := HighestRole([]string{RoleViewer, RoleAdmin}); got != RoleAdmin {
		t.Fatalf("got %q want admin", got)
	}
	if got := HighestRole(nil); !strings.IsEmpty(got) {
		t.Fatalf("got %q want empty", got)
	}
}
