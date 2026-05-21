// Copyright RetailNext, Inc. 2026

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestAccTableGrantsResourceImport(t *testing.T) {
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
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "id", "cycling.cyclist_name"),
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "keyspace", "cycling"),
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "table", "cyclist_name"),
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "grant.#", "1"),
				),
			},
			// Import — single role/privilege so ImportStateVerify can do an exact state comparison
			{
				ResourceName:      "scylladb_table_grants.cyclist_name",
				ImportState:       true,
				ImportStateId:     "cycling.cyclist_name",
				ImportStateVerify: true,
			},
			// Applying the same config after import produces no changes
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("scylladb_table_grants.cyclist_name", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

func TestAccTableGrantsResourceImportMultiRole(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	config := providerConfig + `
resource "scylladb_role" "admin" {
  role      = "admin"
  can_login = false
}
resource "scylladb_role" "readonly" {
  role      = "readonly"
  can_login = false
}
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
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "grant.#", "2"),
				),
			},
			// Import — set-based grants match by value so ImportStateVerify works regardless of map iteration order
			{
				ResourceName:      "scylladb_table_grants.cyclist_name",
				ImportState:       true,
				ImportStateId:     "cycling.cyclist_name",
				ImportStateVerify: true,
			},
			// Apply original config after import; verify final state
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "grant.#", "2"),
				),
			},
		},
	})
}

func TestAccTableGrantsResourceDuplicateRole(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "scylladb_table_grants" "cyclist_name" {
  keyspace = "cycling"
  table    = "cyclist_name"
  grant {
    role       = "admin"
    privileges = ["SELECT"]
  }
  grant {
    role       = "admin"
    privileges = ["ALTER"]
  }
}
`,
				ExpectError: regexp.MustCompile(`Duplicate Grant Role`),
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
					resource.TestCheckResourceAttr("scylladb_table_grants.cyclist_name", "grant.#", "1"),
					func(_ *terraform.State) error {
						cluster, err := getTestScyllaClient([]string{devClusterHost})
						if err != nil {
							return fmt.Errorf("failed to create cluster client: %w", err)
						}
						defer cluster.Session.Close()
						perms, err := cluster.GetAllRolePermissionsPerId(scylladb.ParseIdentifier("cycling.cyclist_name"))
						if err != nil {
							return fmt.Errorf("failed to get permissions: %w", err)
						}
						grants := perms["admin"]
						if len(grants) != 1 {
							return fmt.Errorf("expected 1 grant for admin on cycling.cyclist_name, got %d: %v", len(grants), grants)
						}
						if grants[0] != "SELECT" {
							return fmt.Errorf("expected SELECT grant for admin on cycling.cyclist_name, got %q", grants[0])
						}
						return nil
					},
				),
			},
		},
	})
}
