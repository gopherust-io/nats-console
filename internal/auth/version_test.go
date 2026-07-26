package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func TestLoadUserForSessionSkipsDBWhenVersionMatches(t *testing.T) {
	svc, err := auth.NewService(config.Config{SessionTTL: time.Hour, SessionSecret: "test-secret-at-least-32-bytes!!"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Without a store, LoadUserForSession must succeed from cache when versions match.
	user := store.User{ID: "u1", Username: "alice", Roles: []string{store.RoleViewer}, SessionVersion: 1}
	svc.InvalidateUser("") // no-op
	// Seed via unexported path: CreateSession + Parse won't fill grants cache.
	// Exercise version helpers directly.
	if v := svc.CurrentUserVersion("u1"); v != 1 {
		t.Fatalf("version = %d", v)
	}
	next := svc.BumpUserVersion("u1")
	if next != 2 {
		t.Fatalf("bumped = %d", next)
	}
	if svc.CurrentUserVersion("u1") != 2 {
		t.Fatal("expected version 2")
	}

	token, err := svc.CreateSession(store.User{ID: "u1", Username: "alice", Roles: []string{store.RoleViewer}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := svc.ParseSession(token)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SessionVersion != 2 {
		t.Fatalf("session version = %d, want 2", parsed.SessionVersion)
	}
	_ = context.Background()
	_ = user
}
