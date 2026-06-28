package domain

import "time"

type User struct {
	CreatedAt   time.Time    `json:"createdAt"`
	AccessRules *AccessRules `json:"accessRules,omitempty"`
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	Email       string       `json:"email"`
	OIDCSub     string       `json:"oidcSub,omitempty"`
	Roles       []string     `json:"roles"`
	IsRoot      bool         `json:"isRoot"`
}

type UserCreate struct {
	AccessRules  *AccessRules
	Username     string
	Email        string
	Password     string
	OIDCSub      string
	PasswordHash string
	Roles        []string
	IsRoot       bool
}

type UserUpdate struct {
	Email       *string
	Password    *string
	AccessRules *AccessRules
	Roles       []string
	SetRoles    bool
	SetRules    bool
}
