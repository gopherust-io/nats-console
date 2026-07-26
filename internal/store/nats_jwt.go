package store

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func mintUserJWT(ctx context.Context, s *Store, clusterID, accountName string, user NATSAccountUser, userSeed, accountSeed string) (string, error) {
	ukp, err := nkeys.FromSeed([]byte(userSeed))
	if err != nil {
		return "", fmt.Errorf("user seed: %w", err)
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		return "", err
	}
	akp, err := nkeys.FromSeed([]byte(accountSeed))
	if err != nil {
		return "", fmt.Errorf("account seed: %w", err)
	}

	claims := jwt.NewUserClaims(upub)
	claims.Name = user.Name
	for _, tag := range user.Tags {
		if tag != "" {
			claims.Tags.Add(tag)
		}
	}

	userHasPerms := len(user.PubAllow) > 0 || len(user.PubDeny) > 0 || len(user.SubAllow) > 0 || len(user.SubDeny) > 0
	if userHasPerms {
		claims.Permissions = jwt.Permissions{
			Pub: jwt.Permission{Allow: append([]string(nil), user.PubAllow...), Deny: append([]string(nil), user.PubDeny...)},
			Sub: jwt.Permission{Allow: append([]string(nil), user.SubAllow...), Deny: append([]string(nil), user.SubDeny...)},
		}
	} else if s != nil && s.pool != nil {
		group, gerr := s.GetSigningGroup(ctx, clusterID, accountName, user.SigningGroup)
		if gerr == nil && group.Scoped {
			claims.Permissions = jwt.Permissions{
				Pub: jwt.Permission{Allow: group.PubAllow, Deny: group.PubDeny},
				Sub: jwt.Permission{Allow: group.SubAllow, Deny: group.SubDeny},
			}
			if group.MaxData >= 0 {
				claims.Data = group.MaxData
			}
			if group.MaxPayload >= 0 {
				claims.Limits.Payload = group.MaxPayload
			}
			if group.MaxSubs >= 0 {
				claims.Subs = group.MaxSubs
			}
		}
	}

	if user.RespMaxMsgs > 0 || user.RespTTLNs > 0 {
		claims.Resp = &jwt.ResponsePermission{
			MaxMsgs: user.RespMaxMsgs,
			Expires: time.Duration(user.RespTTLNs),
		}
	}

	claims.BearerToken = user.BearerToken
	claims.ProxyRequired = user.ProxyRequired
	for _, ct := range user.AllowedConnectionTypes {
		if ct != "" {
			claims.AllowedConnectionTypes.Add(ct)
		}
	}
	for _, cidr := range user.SrcCIDRs {
		if cidr != "" {
			claims.Src.Add(cidr)
		}
	}
	claims.Locale = user.TimesLocale
	if len(user.TimeRanges) > 0 {
		times := make([]jwt.TimeRange, 0, len(user.TimeRanges))
		for _, tr := range user.TimeRanges {
			if tr.Start == "" && tr.End == "" {
				continue
			}
			times = append(times, jwt.TimeRange{Start: tr.Start, End: tr.End})
		}
		claims.Times = times
	}

	if user.MaxPayload >= 0 {
		claims.Limits.Payload = user.MaxPayload
	}
	if user.MaxSubs >= 0 {
		claims.Subs = user.MaxSubs
	}
	if user.MaxData >= 0 {
		claims.Data = user.MaxData
	}
	if user.JWTLifetimeNs > 0 {
		claims.Expires = time.Now().UTC().Add(time.Duration(user.JWTLifetimeNs)).Unix()
	}

	return claims.Encode(akp)
}
