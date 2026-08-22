package repo

import "github.com/google/uuid"

// newID returns a new UUID v7 string for entity primary keys.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}
