package subjectauth

import (
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"strings"
)

const (
	PermSourceUser         = "user"
	PermSourceSigningGroup = "signing-group"
	PermSourceUnrestricted = "unrestricted"
)

// EffectiveSubjectPerms holds resolved pub/sub allow/deny lists and their origin.
type EffectiveSubjectPerms struct {
	Source   string
	PubAllow []string
	PubDeny  []string
	SubAllow []string
	SubDeny  []string
}

// PermUser carries the pub/sub fields needed to resolve effective permissions.
type PermUser struct {
	PubAllow []string
	PubDeny  []string
	SubAllow []string
	SubDeny  []string
}

// PermGroup carries signing-group pub/sub fields for scoped inheritance.
// goalign:ignore // trailing bool padding is unavoidable
type PermGroup struct {
	PubAllow []string
	PubDeny  []string
	SubAllow []string
	SubDeny  []string
	Scoped   bool
}

func userHasExplicitPerms(u PermUser) bool {
	return len(u.PubAllow) > 0 || len(u.PubDeny) > 0 || len(u.SubAllow) > 0 || len(u.SubDeny) > 0
}

// ResolveEffectivePerms mirrors mintUserJWT permission resolution.
func ResolveEffectivePerms(user PermUser, group *PermGroup) EffectiveSubjectPerms {
	if userHasExplicitPerms(user) {
		return EffectiveSubjectPerms{
			PubAllow: append([]string(nil), user.PubAllow...),
			PubDeny:  append([]string(nil), user.PubDeny...),
			SubAllow: append([]string(nil), user.SubAllow...),
			SubDeny:  append([]string(nil), user.SubDeny...),
			Source:   PermSourceUser,
		}
	}
	if group != nil && group.Scoped {
		return EffectiveSubjectPerms{
			PubAllow: append([]string(nil), group.PubAllow...),
			PubDeny:  append([]string(nil), group.PubDeny...),
			SubAllow: append([]string(nil), group.SubAllow...),
			SubDeny:  append([]string(nil), group.SubDeny...),
			Source:   PermSourceSigningGroup,
		}
	}
	return EffectiveSubjectPerms{Source: PermSourceUnrestricted}
}

// Permitted reports whether subject is allowed under NATS JWT pub/sub rules.
func Permitted(subject string, allow, deny []string) (allowed bool, matchedPattern string) {
	subject = strings.TrimSpace(subject)
	if commonstrings.IsEmpty(subject) {
		return false, ""
	}
	for _, pattern := range deny {
		if MatchesPattern(subject, pattern) {
			return false, pattern
		}
	}
	if len(allow) == 0 {
		return true, ""
	}
	for _, pattern := range allow {
		if MatchesPattern(subject, pattern) {
			return true, pattern
		}
	}
	return false, ""
}
