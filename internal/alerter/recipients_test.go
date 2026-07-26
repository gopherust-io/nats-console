package alerter

import (
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestRecipientFilterSkipsPlaceholdersAndDedupes(t *testing.T) {
	t.Parallel()
	users := []store.User{
		{Email: "admin@local", IsRoot: true},
		{Email: "", Roles: []string{store.RoleAdmin}},
		{Email: "ops@example.com", IsRoot: true},
		{Email: "OPS@example.com", IsRoot: true},
	}
	seen := map[string]struct{}{}
	var out []string
	for _, user := range users {
		email := strings.TrimSpace(user.Email)
		if mail.IsPlaceholderEmail(email) {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, email)
	}
	assert.Equal(t, []string{"ops@example.com"}, out)
}
