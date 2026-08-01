package subjectauth

import "testing"

func TestPermitted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		match   string
		allow   []string
		deny    []string
		want    bool
	}{
		{
			name:    "unrestricted empty allow",
			subject: "orders.new",
			want:    true,
		},
		{
			name:    "allow match",
			subject: "orders.new",
			allow:   []string{"orders.>"},
			want:    true,
			match:   "orders.>",
		},
		{
			name:    "allow no match",
			subject: "orders.new",
			allow:   []string{"billing.>"},
			want:    false,
		},
		{
			name:    "deny overrides allow",
			subject: "orders.admin.x",
			allow:   []string{"orders.>"},
			deny:    []string{"orders.admin.>"},
			want:    false,
			match:   "orders.admin.>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, match := Permitted(tc.subject, tc.allow, tc.deny)
			if got != tc.want || match != tc.match {
				t.Fatalf("Permitted(%q, %v, %v) = (%v, %q), want (%v, %q)",
					tc.subject, tc.allow, tc.deny, got, match, tc.want, tc.match)
			}
		})
	}
}

func TestResolveEffectivePerms(t *testing.T) {
	t.Parallel()

	userExplicit := PermUser{
		PubAllow: []string{"a.>"},
		SubAllow: []string{"b.>"},
	}
	got := ResolveEffectivePerms(userExplicit, nil)
	if got.Source != PermSourceUser || got.PubAllow[0] != "a.>" {
		t.Fatalf("user explicit: %+v", got)
	}

	group := &PermGroup{Scoped: true, PubAllow: []string{"g.>"}, SubAllow: []string{"h.>"}}
	got = ResolveEffectivePerms(PermUser{}, group)
	if got.Source != PermSourceSigningGroup {
		t.Fatalf("group scoped: %+v", got)
	}

	got = ResolveEffectivePerms(PermUser{}, &PermGroup{Scoped: false})
	if got.Source != PermSourceUnrestricted {
		t.Fatalf("unrestricted: %+v", got)
	}
}
