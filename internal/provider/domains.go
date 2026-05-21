// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

// grantModel is shared by keyspaceGrantsResource and tableGrantsResource.
type grantModel struct {
	Role       types.String `tfsdk:"role"`
	Privileges types.Set    `tfsdk:"privileges"`
}

type Grants []grantModel

func GetGrantsFromRoleBindings(ctx context.Context, bindings []scylladb.AuthoritativeBinding) (Grants, diag.Diagnostics) {
	var grants Grants
	for _, b := range bindings {
		privSet, diags := types.SetValueFrom(ctx, types.StringType, b.Privileges)
		if diags.HasError() {
			return nil, diags
		}
		grants = append(grants, grantModel{
			Role:       types.StringValue(b.Role),
			Privileges: privSet,
		})
	}
	return grants, nil
}
