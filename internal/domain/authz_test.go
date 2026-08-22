package domain

import "testing"

func TestCanDispatcher(t *testing.T) {
	t.Parallel()
	root := User{IsRoot: true}
	if !Can(root, ActionWrite, Resource{}) {
		t.Fatal("root should Can write")
	}
	viewer := User{Roles: []string{RoleViewer}}
	if Can(viewer, ActionWrite, Resource{}) {
		t.Fatal("viewer should not Can write")
	}
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	scoped := User{
		Roles:       []string{RoleViewer},
		AccessRules: &AccessRules{ClusterIDs: []string{clusterID}},
	}
	if !Can(scoped, ActionAccessCluster, Resource{ClusterID: clusterID}) {
		t.Fatal("scoped viewer should access cluster")
	}
}
