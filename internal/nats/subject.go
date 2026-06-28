package natsclient

import "strings"

// SubjectMatchesPattern reports whether subject matches a NATS subject pattern (including * and >).
func SubjectMatchesPattern(subject, pattern string) bool {
	subject = strings.TrimSpace(subject)
	pattern = strings.TrimSpace(pattern)
	if subject == "" || pattern == "" {
		return false
	}
	subTokens := strings.Split(subject, ".")
	patTokens := strings.Split(pattern, ".")
	return subjectMatchTokens(subTokens, patTokens)
}

func subjectMatchTokens(sub, pat []string) bool {
	for len(pat) > 0 {
		if len(pat) == 1 && pat[0] == ">" {
			return len(sub) > 0
		}
		if len(sub) == 0 {
			return false
		}
		switch pat[0] {
		case "*":
			sub, pat = sub[1:], pat[1:]
		case ">":
			return true
		default:
			if sub[0] != pat[0] {
				return false
			}
			sub, pat = sub[1:], pat[1:]
		}
	}
	return len(sub) == 0
}

func ResolvePublishSubject(requested string, streamSubjects []string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
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
