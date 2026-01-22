// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scylladb

import (
	"fmt"
	"testing"

	"github.com/i1snow/terraform-provider-scylladb/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// newTestCluster creates a Cluster connected to a test ScyllaDB container.
func newTestCluster(t *testing.T) *Cluster {
	host := testutil.NewTestContainer(t)
	cluster := NewClusterConfig([]string{host})
	cluster.SetSystemAuthKeyspace("system")
	cluster.SetUserPasswordAuth("cassandra", "cassandra")
	if err := cluster.CreateSession(); err != nil {
		t.Fatalf("failed to create session: %s", err)
	}
	return &cluster
}

func TestGetRoleCassandra(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	role, err := cluster.GetRole("cassandra")
	if err != nil {
		t.Fatalf("failed to get role: %s", err)
	}

	expectedRole := Role{
		Role:        "cassandra",
		CanLogin:    true,
		IsSuperuser: true,
		MemberOf:    nil,
	}

	assert.Equal(t, expectedRole, role)
}

func TestCreateRole(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	inputRole := Role{
		Role: "testRole",
	}
	expectedRole := Role{
		Role:        "testRole",
		IsSuperuser: false,
		CanLogin:    false,
		MemberOf:    nil,
	}

	err := cluster.CreateRole(inputRole)
	if err != nil {
		t.Fatalf("failed to create a role: %s", err)
	}

	role, err := cluster.GetRole(inputRole.Role)
	if err != nil {
		t.Fatalf("failed to get a role for %s: %s", inputRole.Role, err)
	}

	assert.Equal(t, expectedRole, role)
}

func TestUpdateRole(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	inputRole := Role{
		Role: "testRole",
	}
	updateRole := Role{
		Role:        "testRole",
		IsSuperuser: true,
		CanLogin:    true,
	}
	expectedRole := Role{
		Role:        "testRole",
		IsSuperuser: true,
		CanLogin:    true,
		MemberOf:    nil,
	}

	err := cluster.CreateRole(inputRole)
	if err != nil {
		t.Fatalf("failed to create a role: %s", err)
	}

	err = cluster.UpdateRole(updateRole)
	if err != nil {
		t.Fatalf("failed to update a role: %s", err)
	}

	role, err := cluster.GetRole(inputRole.Role)
	if err != nil {
		t.Fatalf("failed to get a role for %s: %s", inputRole.Role, err)
	}

	assert.Equal(t, expectedRole, role)
}

func TestDeleteRole(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	inputRole := Role{
		Role: "testRole",
	}

	err := cluster.CreateRole(inputRole)
	if err != nil {
		t.Fatalf("failed to create a role: %s", err)
	}

	role, err := cluster.GetRole(inputRole.Role)
	if err != nil {
		t.Fatalf("failed to get a role for %s: %s", inputRole.Role, err)
	}
	fmt.Println(role)

	err = cluster.DeleteRole(inputRole)
	if err != nil {
		t.Fatalf("failed to delete a role: %s", err)
	}

	_, err = cluster.GetRole(inputRole.Role)
	assert.EqualError(t, err, "not found")
}
