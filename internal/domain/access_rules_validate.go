package domain

import "errors"

// ValidateAccessRules ensures access rules are internally consistent.
func ValidateAccessRules(rules *AccessRules) error {
	if rules == nil {
		return nil
	}
	if len(rules.ClusterIDs) == 0 {
		return errors.New("clusterIds required: assign at least one cluster")
	}
	for _, role := range rules.AssignableRoles {
		switch role {
		case RoleAdmin, RoleOperator, RoleViewer:
		default:
			return errors.New("invalid assignable role: " + role)
		}
	}
	if rules.ManageUsers && len(rules.AssignableRoles) == 0 {
		return errors.New("assignableRoles required when manageUsers is true")
	}
	return nil
}
