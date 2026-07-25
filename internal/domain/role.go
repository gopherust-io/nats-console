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

// HighestRole returns the highest privilege role in roles, or "".
func HighestRole(roles []string) string {
	if HasRole(roles, RoleAdmin) {
		return RoleAdmin
	}
	if HasRole(roles, RoleOperator) {
		return RoleOperator
	}
	if HasRole(roles, RoleViewer) {
		return RoleViewer
	}
	return ""
}
