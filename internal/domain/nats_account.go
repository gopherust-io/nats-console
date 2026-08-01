package domain

import "time"

type NATSUserTimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// goalign:ignore
type NATSAccountUser struct {
	CreatedAt              time.Time           `json:"createdAt"`
	UpdatedAt              time.Time           `json:"updatedAt"`
	ID                     string              `json:"id"`
	ClusterID              string              `json:"clusterId"`
	AccountName            string              `json:"accountName"`
	Name                   string              `json:"name"`
	PublicKey              string              `json:"publicKey"`
	SigningGroup           string              `json:"signingGroup"`
	AssignedUserID         string              `json:"assignedUserId,omitempty"`
	Tags                   []string            `json:"tags,omitempty"`
	PubAllow               []string            `json:"pubAllow,omitempty"`
	PubDeny                []string            `json:"pubDeny,omitempty"`
	SubAllow               []string            `json:"subAllow,omitempty"`
	SubDeny                []string            `json:"subDeny,omitempty"`
	AllowedConnectionTypes []string            `json:"allowedConnectionTypes,omitempty"`
	SrcCIDRs               []string            `json:"srcCidrs,omitempty"`
	TimesLocale            string              `json:"timesLocale,omitempty"`
	TimeRanges             []NATSUserTimeRange `json:"timeRanges,omitempty"`
	MaxSubs                int64               `json:"maxSubs"`
	MaxPayload             int64               `json:"maxPayload"`
	MaxData                int64               `json:"maxData"`
	JWTLifetimeNs          int64               `json:"jwtLifetimeNs"`
	RespMaxMsgs            int                 `json:"respMaxMsgs"`
	RespTTLNs              int64               `json:"respTTLNs"`
	BearerToken            bool                `json:"bearerToken"`
	ProxyRequired          bool                `json:"proxyRequired"`
	HasJWT                 bool                `json:"hasJwt"`
	JWTIssued              bool                `json:"jwtIssued"`
}

// goalign:ignore
type NATSAccountUserCreate struct {
	ClusterID              string
	AccountName            string
	Name                   string
	SigningGroup           string
	Tags                   []string
	PubAllow               []string
	PubDeny                []string
	SubAllow               []string
	SubDeny                []string
	AllowedConnectionTypes []string
	SrcCIDRs               []string
	TimesLocale            string
	TimeRanges             []NATSUserTimeRange
	MaxSubs                int64
	MaxPayload             int64
	MaxData                int64
	JWTLifetimeNs          int64
	RespMaxMsgs            int
	RespTTLNs              int64
	BearerToken            bool
	ProxyRequired          bool
}

// goalign:ignore
type NATSAccountUserUpdate struct {
	SigningGroup           string
	Tags                   []string
	PubAllow               []string
	PubDeny                []string
	SubAllow               []string
	SubDeny                []string
	AllowedConnectionTypes []string
	SrcCIDRs               []string
	TimesLocale            string
	TimeRanges             []NATSUserTimeRange
	MaxSubs                int64
	MaxPayload             int64
	MaxData                int64
	JWTLifetimeNs          int64
	RespMaxMsgs            int
	RespTTLNs              int64
	BearerToken            bool
	ProxyRequired          bool
}

//nolint:govet // fieldalignment conflicts with embedded-first rule for NATSAccountUser
type NATSAccountUserCreds struct {
	NATSAccountUser

	Seed string `json:"seed,omitempty"`
	JWT  string `json:"jwt,omitempty"`
	Cred string `json:"creds,omitempty"`
}

// goalign:ignore
type SigningGroup struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ID          string    `json:"id"`
	ClusterID   string    `json:"clusterId"`
	AccountName string    `json:"accountName"`
	Name        string    `json:"name"`
	PubAllow    []string  `json:"pubAllow"`
	PubDeny     []string  `json:"pubDeny"`
	SubAllow    []string  `json:"subAllow"`
	SubDeny     []string  `json:"subDeny"`
	MaxData     int64     `json:"maxData"`
	MaxPayload  int64     `json:"maxPayload"`
	MaxSubs     int64     `json:"maxSubs"`
	Scoped      bool      `json:"scoped"`
}

// goalign:ignore
type SigningGroupCreate struct {
	ClusterID   string
	AccountName string
	Name        string
	PubAllow    []string
	PubDeny     []string
	SubAllow    []string
	SubDeny     []string
	MaxData     int64
	MaxPayload  int64
	MaxSubs     int64
	Scoped      bool
}

// goalign:ignore
type SigningGroupUpdate struct {
	PubAllow   []string
	PubDeny    []string
	SubAllow   []string
	SubDeny    []string
	MaxData    int64
	MaxPayload int64
	MaxSubs    int64
	Scoped     bool
}

type NATSAccountExport struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ID          string    `json:"id"`
	ClusterID   string    `json:"clusterId"`
	AccountName string    `json:"accountName"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
}

type NATSAccountExportCreate struct {
	ClusterID   string
	AccountName string
	Kind        string
	Name        string
	Subject     string
	Description string
}

type NATSAccountExportUpdate struct {
	Name        string
	Subject     string
	Description string
}

type SubjectPermissionEntry struct {
	UserID         string `json:"userId"`
	Name           string `json:"name"`
	SigningGroup   string `json:"signingGroup"`
	AssignedUserID string `json:"assignedUserId,omitempty"`
	Source         string `json:"source"`
	MatchedPattern string `json:"matchedPattern,omitempty"`
}

type SubjectPermissionsResult struct {
	Subject        string                   `json:"subject"`
	Publish        []SubjectPermissionEntry `json:"publish"`
	Subscribe      []SubjectPermissionEntry `json:"subscribe"`
	QueueSubscribe []SubjectPermissionEntry `json:"queueSubscribe"`
}
