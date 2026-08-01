package natsclient

import (
	"strings"

	"github.com/gopherust-io/nats-consol/internal/subjectauth"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// SubjectMatchesPattern reports whether subject matches a NATS subject pattern (including * and >).
func SubjectMatchesPattern(subject, pattern string) bool {
	return subjectauth.MatchesPattern(subject, pattern)
}

func ResolvePublishSubject(requested string, streamSubjects []string) (string, error) {
	requested = strings.TrimSpace(requested)
	if !commonstrings.IsEmpty(requested) {
		for _, pattern := range streamSubjects {
			if SubjectMatchesPattern(requested, pattern) {
				return requested, nil
			}
		}
		return "", ErrSubjectNotInStream
	}
	if len(streamSubjects) == 1 && !strings.ContainsAny(streamSubjects[0], "*>") {
		return streamSubjects[0], nil
	}
	return "", ErrSubjectRequired
}
