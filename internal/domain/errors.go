package domain

import "errors"

var (
	ErrNotFound                 = errors.New("not found")
	ErrForbidden                = errors.New("forbidden")
	ErrRootProtected            = errors.New("root user cannot be modified or deleted")
	ErrRootExists               = errors.New("root user already exists")
	ErrCannotEscalate           = errors.New("cannot grant permissions beyond your own")
	ErrInvalidInput             = errors.New("invalid input")
	ErrInvalidRange             = errors.New("invalid time range")
	ErrAlertNotFound            = errors.New("alert not found")
	ErrAlertRuleNotFound        = errors.New("alert rule not found")
	ErrEventCatalogEntryNotFound = errors.New("event catalog entry not found")
	ErrSigningGroupProtected    = errors.New("default signing group cannot be deleted")
	ErrSigningGroupInUse        = errors.New("signing group is still used by one or more NATS users")
	ErrConflict                 = errors.New("conflict")
)
