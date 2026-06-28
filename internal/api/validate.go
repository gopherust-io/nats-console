package api

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

var (
	resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/]{0,255}$`)
	uuidPattern         = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func validateResourceName(name string) error {
	if name == "" || len(name) > 256 {
		return errors.New("invalid name")
	}
	if !resourceNamePattern.MatchString(name) {
		return errors.New("invalid name: use letters, numbers, dot, dash, underscore, or slash")
	}
	return nil
}

func validateClusterName(name string) error {
	if err := validateResourceName(name); err != nil {
		return err
	}
	if strings.Contains(name, "/") {
		return errors.New("invalid cluster name")
	}
	return nil
}

func validateNATSURL(raw string) error {
	if raw == "" {
		return errors.New("invalid nats url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid nats url")
	}
	switch strings.ToLower(u.Scheme) {
	case "nats", "tls", "ws", "wss":
		return nil
	default:
		return errors.New("invalid nats url scheme")
	}
}

func validateHTTPURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid monitoring url")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	default:
		return errors.New("invalid monitoring url scheme")
	}
}

func validateUUID(id string) error {
	if !uuidPattern.MatchString(id) {
		return errors.New("invalid id")
	}
	return nil
}

func validateRoles(roles []string) error {
	if len(roles) == 0 {
		return errors.New("roles required")
	}
	for _, role := range roles {
		switch role {
		case domain.RoleAdmin, domain.RoleOperator, domain.RoleViewer:
		default:
			return errors.New("invalid role: " + role)
		}
	}
	return nil
}
