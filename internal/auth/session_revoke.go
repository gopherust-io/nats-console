package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// sessionRevocations is an in-process denylist of logged-out JWTs.
// Multi-instance deployments do not share this map; prefer short SessionTTL
// or an external session store for stronger logout guarantees.
type sessionRevocations struct {
	entries map[string]time.Time
	ops     uint64
	mu      sync.Mutex
}

func newSessionRevocations() *sessionRevocations {
	return &sessionRevocations{entries: make(map[string]time.Time)}
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (r *sessionRevocations) Revoke(token string, until time.Time) {
	if token == "" || until.IsZero() {
		return
	}
	key := hashSessionToken(token)
	r.mu.Lock()
	r.entries[key] = until
	r.ops++
	if r.ops%256 == 0 {
		r.purgeExpiredLocked(time.Now())
	}
	r.mu.Unlock()
}

func (r *sessionRevocations) IsRevoked(token string) bool {
	if token == "" {
		return false
	}
	key := hashSessionToken(token)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	exp, ok := r.entries[key]
	if !ok {
		return false
	}
	if now.After(exp) {
		delete(r.entries, key)
		return false
	}
	return true
}

func (r *sessionRevocations) purgeExpiredLocked(now time.Time) {
	for key, exp := range r.entries {
		if now.After(exp) {
			delete(r.entries, key)
		}
	}
}
