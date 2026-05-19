// Copyright RetailNext, Inc. 2026

package provider

import (
	"fmt"
	"regexp"
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

func TestAccKeyspaceGrantsResourceImport(t *testing.T) {
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
			// Create
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "id", "cycling"),
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "keyspace", "cycling"),
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "grant.#", "1"),
					resource.TestCheckResourceAttrSet("scylladb_keyspace_grants.cycling", "permissions.#"),
				),
			},
			// Import — single role/privilege so ImportStateVerify can do an exact state comparison
			{
				ResourceName:      "scylladb_keyspace_grants.cycling",
				ImportState:       true,
				ImportStateId:     "cycling",
				ImportStateVerify: true,
			},
			// Applying the same config after import produces no changes
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("scylladb_keyspace_grants.cycling", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

func TestAccKeyspaceGrantsResourceImportMultiRole(t *testing.T) {
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
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with two roles
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "grant.#", "2"),
					resource.TestCheckResourceAttrSet("scylladb_keyspace_grants.cycling", "permissions.#"),
				),
			},
			// Import — grants are sorted by role in ImportState so list order matches config order
			{
				ResourceName:      "scylladb_keyspace_grants.cycling",
				ImportState:       true,
				ImportStateId:     "cycling",
				ImportStateVerify: true,
			},
			// Apply original config after import; verify final state
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "grant.#", "2"),
					resource.TestCheckResourceAttr("scylladb_keyspace_grants.cycling", "permissions.#", "3"),
				),
			},
		},
	})
}

func TestAccKeyspaceGrantsResourceDuplicateRole(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)
	setupTestKeyspaceAndTable(t, []string{devClusterHost})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "scylladb_keyspace_grants" "cycling" {
  keyspace = "cycling"
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
