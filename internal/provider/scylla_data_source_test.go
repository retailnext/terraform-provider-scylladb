// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/i1snow/terraform-provider-scylladb/internal/testutil"
)

func TestAccRoleDataSource(t *testing.T) {
	devClusterHost := testutil.NewTestContainer(t)
	dataConfig := fmt.Sprintf(providerConfigFmt, devClusterHost) + `
data "scylladb_role" "cassandra" {
  id = "cassandra"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: dataConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify cassandra role
					resource.TestCheckResourceAttr("data.scylladb_role.cassandra", "role", "cassandra"),
					resource.TestCheckResourceAttr("data.scylladb_role.cassandra", "can_login", "true"),
					resource.TestCheckResourceAttr("data.scylladb_role.cassandra", "is_superuser", "true"),
				),
			},
		},
	})
}
