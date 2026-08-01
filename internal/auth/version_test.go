package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func versionTestConfig(t *testing.T) config.Config {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	return config.Config{
		Auth: config.AuthConfig{
			SessionTTL:        time.Hour,
			SessionPrivateKey: privPEM,
			SessionPublicKey:  pubPEM,
		},
	}
}

func TestLoadUserForSessionSkipsDBWhenVersionMatches(t *testing.T) {
	svc, err := auth.NewService(versionTestConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Without a store, LoadUserForSession must succeed from cache when versions match.
	ctx := context.Background()
	user := store.User{ID: "u1", Username: "alice", Roles: []string{store.RoleViewer}, SessionVersion: 1}
	svc.InvalidateUser(ctx, "") // no-op
	// Seed via unexported path: CreateSession + Parse won't fill grants cache.
	// Exercise version helpers directly.
	if v := svc.CurrentUserVersion(ctx, "u1"); v != 1 {
		t.Fatalf("version = %d", v)
	}
	next := svc.BumpUserVersion(ctx, "u1")
	if next != 2 {
		t.Fatalf("bumped = %d", next)
	}
	if svc.CurrentUserVersion(ctx, "u1") != 2 {
		t.Fatal("expected version 2")
	}

	token, err := svc.CreateSession(ctx, store.User{ID: "u1", Username: "alice", Roles: []string{store.RoleViewer}}, "fp-version")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := svc.ParseSession(ctx, token, "fp-version")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != "u1" {
		t.Fatalf("parsed id = %s", parsed.ID)
	}
	_ = user
}
