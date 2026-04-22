// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantKS   string
		wantTbl  string
	}{
		{"cycling", "KEYSPACE", "cycling", ""},
		{"cycling.cyclist_name", "TABLE", "cycling", "cyclist_name"},
		{"ks.tbl.extra", "TABLE", "ks", "tbl.extra"}, // only the first dot splits
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseIdentifier(tc.input)
			assert.Equal(t, tc.wantType, got.ResourceType)
			assert.Equal(t, tc.wantKS, got.Keyspace)
			assert.Equal(t, tc.wantTbl, got.Table)
			assert.Equal(t, tc.input, got.Original)
		})
	}
}

func TestApplyAuthoritativeGrant(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	setupTestKSAndTable(t, cluster)

	require.NoError(t, cluster.CreateRole(Role{Role: "role_a"}))
	require.NoError(t, cluster.CreateRole(Role{Role: "role_b"}))

	id := ParseIdentifier("cycling.cyclist_name")
	bindings := []AuthoritativeBinding{
		{Privileges: []string{"ALTER", "SELECT"}, Role: "role_a"},
		{Privileges: []string{"SELECT"}, Role: "role_b"},
	}

	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, bindings))

	perms, err := cluster.GetEffectivePermissions(id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"cycling.cyclist_name:role_a:ALTER",
		"cycling.cyclist_name:role_a:SELECT",
		"cycling.cyclist_name:role_b:SELECT",
	}, perms)
}

func TestApplyAuthoritativeGrantRevokesExcess(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	setupTestKSAndTable(t, cluster)

	require.NoError(t, cluster.CreateRole(Role{Role: "role_c"}))
	require.NoError(t, cluster.CreateRole(Role{Role: "role_d"}))

	id := ParseIdentifier("cycling.cyclist_name")

	// Grant an initial wide set of permissions to both roles
	initial := []AuthoritativeBinding{
		{Privileges: []string{"ALTER", "MODIFY", "SELECT"}, Role: "role_c"},
		{Privileges: []string{"ALTER", "MODIFY", "SELECT"}, Role: "role_d"},
	}
	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, initial))

	// Now apply a narrower set — excess should be revoked and role_d removed entirely
	narrowed := []AuthoritativeBinding{
		{Privileges: []string{"SELECT"}, Role: "role_c"},
	}
	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, narrowed))

	perms, err := cluster.GetEffectivePermissions(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"cycling.cyclist_name:role_c:SELECT"}, perms)
}

func TestRevokeAllGrantsOnIdentifier(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	setupTestKSAndTable(t, cluster)

	require.NoError(t, cluster.CreateRole(Role{Role: "role_e"}))

	id := ParseIdentifier("cycling.cyclist_name")
	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, []AuthoritativeBinding{
		{Privileges: []string{"SELECT"}, Role: "role_e"},
	}))

	require.NoError(t, cluster.RevokeAllGrantsOnIdentifier(id))

	perms, err := cluster.GetEffectivePermissions(id)
	require.NoError(t, err)
	assert.Empty(t, perms)
}

func TestApplyAuthoritativeGrantKeyspace(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	setupTestKSAndTable(t, cluster)

	require.NoError(t, cluster.CreateRole(Role{Role: "role_f"}))

	id := ParseIdentifier("cycling")
	bindings := []AuthoritativeBinding{
		{Privileges: []string{"SELECT", "MODIFY"}, Role: "role_f"},
	}

	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, bindings))

	perms, err := cluster.GetEffectivePermissions(id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"cycling:role_f:SELECT",
		"cycling:role_f:MODIFY",
	}, perms)
}

func TestApplyAuthoritativeGrantCaseInsensitive(t *testing.T) {
	cluster := newTestCluster(t)
	defer cluster.Session.Close()

	setupTestKSAndTable(t, cluster)

	require.NoError(t, cluster.CreateRole(Role{Role: "role_g"}))

	id := ParseIdentifier("cycling.cyclist_name")

	// Grant using mixed-case privilege names
	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, []AuthoritativeBinding{
		{Privileges: []string{"select", "Modify"}, Role: "role_g"},
	}))

	// Re-apply with uppercase — should be a no-op (no revoke/re-grant)
	require.NoError(t, cluster.ApplyAuthoritativeGrant(id, []AuthoritativeBinding{
		{Privileges: []string{"SELECT", "MODIFY"}, Role: "role_g"},
	}))

	perms, err := cluster.GetEffectivePermissions(id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"cycling.cyclist_name:role_g:SELECT",
		"cycling.cyclist_name:role_g:MODIFY",
	}, perms)
}

func TestAuthoritativeBindingValidate(t *testing.T) {
	tests := []struct {
		name      string
		binding   AuthoritativeBinding
		wantError bool
	}{
		{
			name:      "valid privileges",
			binding:   AuthoritativeBinding{Privileges: []string{"SELECT", "MODIFY"}, Role: "role_a"},
			wantError: false,
		},
		{
			name:      "invalid privilege ALL PERMISSIONS uppercase",
			binding:   AuthoritativeBinding{Privileges: []string{"ALL PERMISSIONS"}, Role: "role_a"},
			wantError: true,
		},
		{
			name:      "invalid privilege ALL PERMISSIONS lowercase",
			binding:   AuthoritativeBinding{Privileges: []string{"all permissions"}, Role: "role_a"},
			wantError: true,
		},
		{
			name:      "invalid privilege mixed with valid",
			binding:   AuthoritativeBinding{Privileges: []string{"SELECT", "ALL PERMISSIONS"}, Role: "role_a"},
			wantError: true,
		},
		{
			name:      "empty privileges",
			binding:   AuthoritativeBinding{Privileges: []string{}, Role: "role_a"},
			wantError: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.binding.Validate()
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// setupTestKSAndTable creates the cycling keyspace and cycling.cyclist_name table if they don't exist.
func setupTestKSAndTable(t *testing.T, cluster *Cluster) {
	t.Helper()
	err := cluster.CreateKeyspace(Keyspace{
		Name:              "cycling",
		ReplicationClass:  "SimpleStrategy",
		ReplicationFactor: 1,
		DurableWrites:     true,
	})
	require.NoError(t, err)

	require.NoError(t, cluster.Session.Query(`
		CREATE TABLE IF NOT EXISTS cycling.cyclist_name (
			id UUID PRIMARY KEY,
			cyclist_name text
		)`).Exec())
}
