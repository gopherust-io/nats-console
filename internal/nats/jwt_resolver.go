package natsclient

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
)

type ParsedAccountJWT struct {
	ExpiresAt *time.Time
	Name      string
}

func ParseAccountJWT(raw string) (ParsedAccountJWT, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedAccountJWT{}, errors.New("jwt is required")
	}
	ac, err := jwt.DecodeAccountClaims(raw)
	if err != nil {
		return ParsedAccountJWT{}, fmt.Errorf("decode account jwt: %w", err)
	}
	name := ac.Name
	if name == "" {
		name = ac.Subject
	}
	out := ParsedAccountJWT{Name: name}
	if ac.Expires > 0 {
		t := time.Unix(ac.Expires, 0).UTC()
		out.ExpiresAt = &t
	}
	return out, nil
}
