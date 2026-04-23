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

func getTestScyllaClient(hosts []string) (*scylladb.Cluster, error) {
	cluster, err := scylladb.NewClusterConfig(hosts)
	if err != nil {
		return nil, err
	}
	cluster.SetSystemAuthKeyspace("system")
	cluster.SetUserPasswordAuth("cassandra", "cassandra")
	if err = cluster.CreateSession(); err != nil {
		return nil, err
	}
	return cluster, nil
}

// TestAccRoleResourceExternallyDeleted verifies that when a role is deleted outside of Terraform,
// running plan does not error out and instead plans to recreate the role.
func TestAccRoleResourceExternallyDeleted(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)

	roleConfig := providerConfig + `
resource "scylladb_role" "admin" {
    role = "admin"
    can_login = false
    is_superuser = false
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create the role
			{
				Config: roleConfig,
				Check:  resource.TestCheckResourceAttr("scylladb_role.admin", "role", "admin"),
			},
			// Delete the role externally, then verify plan recovers without error and plans to recreate
			{
				PreConfig: func() {
					cluster, err := getTestScyllaClient([]string{devClusterHost})
					if err != nil {
						t.Fatalf("failed to create cluster config: %s", err)
					}
					defer cluster.Session.Close()
					if err := cluster.DeleteRole(scylladb.Role{Role: "admin"}); err != nil {
						t.Fatalf("failed to delete role externally: %s", err)
					}
				},
				Config: roleConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("scylladb_role.admin", plancheck.ResourceActionCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_role.admin", "role", "admin"),
					resource.TestCheckResourceAttr("scylladb_role.admin", "can_login", "false"),
					resource.TestCheckResourceAttr("scylladb_role.admin", "is_superuser", "false"),
					resource.TestCheckResourceAttrSet("scylladb_role.admin", "id"),
				),
			},
		},
	})
}

func TestAccRoleResource(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	providerConfig := fmt.Sprintf(providerConfigFmt, devClusterHost)

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
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_role.admin", "role", "admin"),
					resource.TestCheckResourceAttr("scylladb_role.admin", "can_login", "false"),
					resource.TestCheckResourceAttr("scylladb_role.admin", "is_superuser", "false"),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("scylladb_role.admin", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "scylladb_role.admin",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "scylladb_role" "admin" {
    role = "admin"
    can_login = true
    is_superuser = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("scylladb_role.admin", "role", "admin"),
					resource.TestCheckResourceAttr("scylladb_role.admin", "can_login", "true"),
					resource.TestCheckResourceAttr("scylladb_role.admin", "is_superuser", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
