package repo

import (
	"context"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/subjectauth"
)

type (
	SubjectPermissionEntry   = domain.SubjectPermissionEntry
	SubjectPermissionsResult = domain.SubjectPermissionsResult
)

func (db *DB) ListSubjectPermissions(ctx context.Context, clusterID, accountName, subject string) (SubjectPermissionsResult, error) {
	subject = strings.TrimSpace(subject)
	result := SubjectPermissionsResult{Subject: subject}

	users, err := db.ListNATSAccountUsers(ctx, clusterID, accountName)
	if err != nil {
		return result, err
	}
	groups, err := db.ListSigningGroups(ctx, clusterID, accountName)
	if err != nil {
		return result, err
	}
	groupByName := make(map[string]SigningGroup, len(groups))
	for _, g := range groups {
		groupByName[g.Name] = g
	}

	for _, user := range users {
		var groupPtr *subjectauth.PermGroup
		if g, ok := groupByName[user.SigningGroup]; ok {
			groupPtr = &subjectauth.PermGroup{
				Scoped:   g.Scoped,
				PubAllow: g.PubAllow,
				PubDeny:  g.PubDeny,
				SubAllow: g.SubAllow,
				SubDeny:  g.SubDeny,
			}
		}
		effective := subjectauth.ResolveEffectivePerms(subjectauth.PermUser{
			PubAllow: user.PubAllow,
			PubDeny:  user.PubDeny,
			SubAllow: user.SubAllow,
			SubDeny:  user.SubDeny,
		}, groupPtr)

		entry := SubjectPermissionEntry{
			UserID:         user.ID,
			Name:           user.Name,
			SigningGroup:   user.SigningGroup,
			AssignedUserID: user.AssignedUserID,
			Source:         effective.Source,
		}

		if allowed, pattern := subjectauth.Permitted(subject, effective.PubAllow, effective.PubDeny); allowed {
			pub := entry
			pub.MatchedPattern = pattern
			result.Publish = append(result.Publish, pub)
		}
		if allowed, pattern := subjectauth.Permitted(subject, effective.SubAllow, effective.SubDeny); allowed {
			sub := entry
			sub.MatchedPattern = pattern
			result.Subscribe = append(result.Subscribe, sub)
			qs := entry
			qs.MatchedPattern = pattern
			result.QueueSubscribe = append(result.QueueSubscribe, qs)
		}
	}

	if result.Publish == nil {
		result.Publish = []SubjectPermissionEntry{}
	}
	if result.Subscribe == nil {
		result.Subscribe = []SubjectPermissionEntry{}
	}
	if result.QueueSubscribe == nil {
		result.QueueSubscribe = []SubjectPermissionEntry{}
	}
	return result, nil
}
