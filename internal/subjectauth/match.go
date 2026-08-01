package subjectauth

import (
	"strings"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"

	// MatchesPattern reports whether subject matches a NATS subject pattern (including * and >).
)

func MatchesPattern(subject, pattern string) bool {
	subject = strings.TrimSpace(subject)
	pattern = strings.TrimSpace(pattern)
	if commonstrings.IsEmpty(subject) || commonstrings.IsEmpty(pattern) {
		return false
	}
	subTokens := strings.Split(subject, ".")
	patTokens := strings.Split(pattern, ".")
	return matchTokens(subTokens, patTokens)
}

func matchTokens(sub, pat []string) bool {
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
