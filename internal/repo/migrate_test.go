package repo

import "testing"

func TestLegacyMigrationVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"001_clusters", 1, true},
		{"020_event_catalog", 20, true},
		{"000_schema_migrations", 0, false},
		{"", 0, false},
		{"clusters", 0, false},
		{" 016_auth_persistence ", 16, true},
	}
	for _, tc := range cases {
		got, ok := legacyMigrationVersion(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%q: got (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
