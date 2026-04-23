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

func TestAccKeyspaceGrantsResource(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)

	// Set up initial keyspace and table
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
resource "scylladb_keyspace_grants" "cycling" {
  keyspace = "cycling"
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
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "keyspace", "cycling"),
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "grant.#", "2"),
					resource.TestCheckResourceAttrSet("scylladb_keyspace_grants.cycling", "id"),
					resource.TestCheckResourceAttrSet("scylladb_keyspace_grants.cycling", "permissions.#"),
				),
			},
			// Update — narrow grants; excess is revoked
			{
				Config: baseConfig + `
resource "scylladb_keyspace_grants" "cycling" {
  keyspace = "cycling"
  grant {
    role       = scylladb_role.admin.role
    privileges = ["SELECT"]
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "grant.#", "1"),
				),
			},
		},
	})
}

func TestAccKeyspaceGrantsResourceDriftDetection(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	config := providerConfig + `
resource "scylladb_role" "admin" {
  role      = "admin"
  can_login = false
}
resource "scylladb_keyspace_grants" "cycling" {
  keyspace = "cycling"
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
				Check:  resource.TestCheckResourceAttrSet("scylladb_keyspace_grants.cycling", "id"),
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
						ResourceType: "KEYSPACE",
						Keyspace:     "cycling",
					}); err != nil {
						t.Fatalf("failed to add external grant: %s", err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("scylladb_keyspace_grants.cycling", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "permissions.#", "1"),
				),
			},
		},
	})
}
