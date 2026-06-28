package domain

import "slices"

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

func HasRole(roles []string, role string) bool {
	return slices.Contains(roles, role)
}
