package domain

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrRootProtected  = errors.New("root user cannot be modified or deleted")
	ErrRootExists     = errors.New("root user already exists")
	ErrCannotEscalate = errors.New("cannot grant permissions beyond your own")
	ErrInvalidInput   = errors.New("invalid input")
)
