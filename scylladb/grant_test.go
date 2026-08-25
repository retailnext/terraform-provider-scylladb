// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testRole = Role{
		Role:        "testRole",
		CanLogin:    false,
		IsSuperuser: false,
		MemberOf:    nil,
	}
	testKeyspace = Keyspace{
		Name:              "cycling",
		ReplicationClass:  "SimpleStrategy",
		ReplicationFactor: 1,
		DurableWrites:     true,
	}
)

func TestGrantMethods(t *testing.T) {
	cluster := newTestClusterWithTableAndRole(t)
	defer cluster.Session.Close()

	grant := Grant{
		RoleName:     "testRole",
		Privilege:    "select",
		ResourceType: "table",
		Keyspace:     "cycling",
		Identifier:   "cyclist_name",
	}
	err := cluster.CreateGrant(grant)
	if err != nil {
		t.Fatalf("failed to create grant: %s", err)
	}
	permissions, found, err := cluster.ListGrant(grant)
	if err != nil {
		t.Fatalf("failed to get grant: %s", err)
	}

	if !found {
		t.Fatalf("grant not found")
	}

	expectedPermissions := []Permission{
		{
			Role:       "testRole",
			Username:   "testRole",
			Resource:   "<table cycling.cyclist_name>",
			Permission: "SELECT",
		},
	}

	assert.Equal(t, expectedPermissions, permissions)

	permStrs, err := cluster.GetRolePermissions(grant)
	if err != nil {
		t.Fatalf("failed to get grant permission strings: %s", err)
	}
	assert.Equal(t, []string{"SELECT"}, permStrs)

	err = cluster.DeleteGrant(grant)
	if err != nil {
		t.Fatalf("failed to delete grant: %s", err)
	}

	_, found, err = cluster.ListGrant(grant)
	if err != nil {
		t.Fatalf("failed to get grant after deletion: %s", err)
	}

	if found {
		t.Fatalf("grant still found after deletion")
	}
}

func TestDeleteGrantAlreadyGone(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	err := cluster.DeleteGrant(Grant{
		RoleName:     "it_should_not_exist",
		Privilege:    "SELECT",
		ResourceType: "KEYSPACE",
		Keyspace:     "it_should_not_exist_either",
	})
	assert.NoError(t, err)
}

func TestDeleteGrantTwice(t *testing.T) {
	cluster := newTestClusterWithTableAndRole(t)
	defer cluster.Session.Close()

	grant := Grant{
		RoleName:     "testRole",
		Privilege:    "SELECT",
		ResourceType: "TABLE",
		Keyspace:     "cycling",
		Identifier:   "cyclist_name",
	}
	err := cluster.CreateGrant(grant)
	if err != nil {
		t.Fatalf("failed to create grant: %s", err)
	}

	err = cluster.DeleteGrant(grant)
	if err != nil {
		t.Fatalf("failed to delete grant: %s", err)
	}

	// The role and keyspace/table still exist, but the grant itself is already
	// gone. Deleting it again should still be a no-op, not an error.
	err = cluster.DeleteGrant(grant)
	assert.NoError(t, err)
}

func TestGrantPermissions(t *testing.T) {
	cluster := newTestClusterWithTableAndRole(t)
	defer cluster.Session.Close()

	keyspaceGrant := Grant{
		RoleName:     "testRole",
		Privilege:    "SELECT",
		ResourceType: "KEYSPACE",
		Keyspace:     "cycling",
	}
	err := cluster.CreateGrant(keyspaceGrant)
	if err != nil {
		t.Fatalf("failed to create keyspace grant: %s", err)
	}
	tableGrant := Grant{
		RoleName:     "testRole",
		Privilege:    "ALL PERMISSIONS",
		ResourceType: "TABLE",
		Keyspace:     "cycling",
		Identifier:   "cyclist_name",
	}
	err = cluster.CreateGrant(tableGrant)
	if err != nil {
		t.Fatalf("failed to create table grant: %s", err)
	}

	// Get the permissions of tableGrant
	permissions, err := cluster.GetGrantPermissions(tableGrant)
	if err != nil {
		t.Fatalf("failed to get grant: %s", err)
	}

	expectedPermissions := tableGrant.GetExpandedPermissions()
	assert.Equal(t, expectedPermissions, permissions)
}

func newTestClusterWithTableAndRole(t *testing.T) *Cluster {
	cluster := newTestCluster(t)

	if err := cluster.CreateRole(testRole); err != nil {
		t.Fatalf("failed to create role: %s", err)
	}
	if err := cluster.CreateKeyspace(testKeyspace); err != nil {
		t.Fatalf("failed to create keyspace: %s", err)
	}

	createTableQuery := `CREATE TABLE IF NOT EXISTS cycling.cyclist_name (id UUID PRIMARY KEY, name text);`
	if err := cluster.Session.Query(createTableQuery).Exec(); err != nil {
		t.Fatalf("failed to create table: %s", err)
	}

	return cluster
}
