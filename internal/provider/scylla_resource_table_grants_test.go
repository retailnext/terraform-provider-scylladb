// Copyright RetailNext, Inc. 2026

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/retailnext/terraform-provider-scylladb/internal/testutil"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

func TestAccTableGrantsResource(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	baseConfig := providerConfig + `
resource "scylladb_role" "admin" {
  role      = "admin"
  can_login = false
}
resource "scylladb_role" "readonly" {
  role      = "readonly"
  can_login = false
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: baseConfig + `
resource "scylladb_table_grants" "cyclist_name" {
  keyspace = "cycling"
  table    = "cyclist_name"
  grant {
    role       = scylladb_role.admin.role
    privileges = ["ALTER", "SELECT"]
  }
  grant {
    role       = scylladb_role.readonly.role
    privileges = ["SELECT"]
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "keyspace", "cycling"),
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "table", "cyclist_name"),
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "id", "cycling.cyclist_name"),
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "grant.#", "2"),
					resource.TestCheckResourceAttrSet("scylladb_table_grants.cyclist_name", "permissions.#"),
				),
			},
			// Update — narrow grants; excess is revoked
			{
				Config: baseConfig + `
resource "scylladb_table_grants" "cyclist_name" {
  keyspace = "cycling"
  table    = "cyclist_name"
  grant {
    role       = scylladb_role.admin.role
    privileges = ["SELECT"]
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "grant.#", "1"),
				),
			},
		},
	})
}

func TestAccTableGrantsResourceDriftDetection(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	config := providerConfig + `
resource "scylladb_role" "admin" {
  role      = "admin"
  can_login = false
}
resource "scylladb_table_grants" "cyclist_name" {
  keyspace = "cycling"
  table    = "cyclist_name"
  grant {
    role       = scylladb_role.admin.role
    privileges = ["SELECT"]
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttrSet("scylladb_table_grants.cyclist_name", "id"),
			},
			// Externally add a grant, verify plan detects drift and reconciles
			{
				PreConfig: func() {
					cluster, err := getTestScyllaClient([]string{devClusterHost})
					if err != nil {
						t.Fatalf("failed to create cluster client: %s", err)
					}
					defer cluster.Session.Close()
					if err := cluster.CreateGrant(scylladb.Grant{
						RoleName:     "admin",
						Privilege:    "ALTER",
						ResourceType: "TABLE",
						Keyspace:     "cycling",
						Identifier:   "cyclist_name",
					}); err != nil {
						t.Fatalf("failed to add external grant: %s", err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("scylladb_table_grants.cyclist_name", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "permissions.#", "1"),
				),
			},
		},
	})
}
