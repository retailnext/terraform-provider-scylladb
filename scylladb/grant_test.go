// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	permissions, found, err := cluster.GetGrant(grant)
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

	err = cluster.DeleteGrant(grant)
	if err != nil {
		t.Fatalf("failed to delete grant: %s", err)
	}

	_, found, err = cluster.GetGrant(grant)
	if err != nil {
		t.Fatalf("failed to get grant after deletion: %s", err)
	}

	if found {
		t.Fatalf("grant still found after deletion")
	}
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
