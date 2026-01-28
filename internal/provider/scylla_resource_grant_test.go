// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/retailnext/terraform-provider-scylladb/internal/testutil"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

func TestAccGrantResource(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)

	// Set up initial keyspace and table
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "scylladb_role" "admin" {
	role = "admin"
	can_login = false
	is_superuser = false
}
resource "scylladb_grant" "admin_alter_keyspace" {
  role_name = scylladb_role.admin.role
  privilege = "ALTER"
  resource_type = "KEYSPACE"
  keyspace   = "cycling"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_keyspace", "role_name", "admin"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_keyspace", "privilege", "ALTER"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_keyspace", "resource_type", "KEYSPACE"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_keyspace", "keyspace", "cycling"),
					// Verify that optional attribute is not set
					resource.TestCheckNoResourceAttr("scylladb_grant.admin_alter_keyspace", "identifier"),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("scylladb_grant.admin_alter_keyspace", "id"),
					resource.TestCheckResourceAttrSet("scylladb_grant.admin_alter_keyspace", "last_updated"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "scylladb_grant.admin_alter_keyspace",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"last_updated"},
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "scylladb_role" "admin" {
	role = "admin"
	can_login = false
	is_superuser = false
}
resource "scylladb_grant" "admin_alter_cyclist_name" {
  role_name = scylladb_role.admin.role
  privilege = "ALTER"
  resource_type = "TABLE"
  keyspace   = "cycling"
  identifier = "cyclist_name"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_cyclist_name", "role_name", "admin"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_cyclist_name", "privilege", "ALTER"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_cyclist_name", "resource_type", "TABLE"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_cyclist_name", "keyspace", "cycling"),
					resource.TestCheckResourceAttr("scylladb_grant.admin_alter_cyclist_name", "identifier", "cyclist_name"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func setupTestKeyspaceAndTable(t *testing.T, hosts []string) {
	cluster := scylladb.NewClusterConfig(hosts)
	cluster.SetSystemAuthKeyspace("system")
	cluster.SetUserPasswordAuth("cassandra", "cassandra")
	if err := cluster.CreateSession(); err != nil {
		t.Fatalf("failed to create session: %s", err)
	}
	defer cluster.Session.Close()
	err := cluster.CreateKeyspace(scylladb.Keyspace{
		Name:              "cycling",
		ReplicationClass:  "SimpleStrategy",
		ReplicationFactor: 1,
		DurableWrites:     true,
	})
	if err != nil {
		t.Fatalf("failed to create keyspace: %s", err)
	}

	createTableCQL := `
	CREATE TABLE IF NOT EXISTS cycling.cyclist_name (
		id UUID PRIMARY KEY,
		cyclist_name text
	);`
	if err := cluster.Session.Query(createTableCQL).Exec(); err != nil {
		t.Fatalf("failed to create table: %s", err)
	}
}
