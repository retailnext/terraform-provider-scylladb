// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/i1snow/terraform-provider-scylladb/internal/testutil"
)

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
					resource.TestCheckResourceAttrSet("scylladb_role.admin", "last_updated"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "scylladb_role.admin",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"last_updated"},
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
