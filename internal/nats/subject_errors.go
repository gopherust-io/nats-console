package natsclient

import "errors"

var (
	ErrSubjectNotInStream = errors.New("subject does not match stream subjects")
	ErrSubjectRequired    = errors.New("subject is required for this stream")
)
