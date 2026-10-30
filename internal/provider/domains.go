// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

// grantModel is shared by keyspaceGrantsResource and tableGrantsResource.
type grantModel struct {
	Role       types.String `tfsdk:"role"`
	Privileges types.Set    `tfsdk:"privileges"`
}

// grantObjectType is the element type of the `grant` block. It must match the
// block's schema in keyspaceGrantsResource and tableGrantsResource.
var grantObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"role":       types.StringType,
		"privileges": types.SetType{ElemType: types.StringType},
	},
}

// GetGrantsFromRoleBindings converts role bindings into a `grant` set value.
// No bindings yield an empty set, not a null one.
func GetGrantsFromRoleBindings(ctx context.Context, bindings []scylladb.AuthoritativeBinding) (types.Set, diag.Diagnostics) {
	grants := make([]grantModel, 0, len(bindings))
	for _, b := range bindings {
		privSet, diags := types.SetValueFrom(ctx, types.StringType, b.Privileges)
		if diags.HasError() {
			return types.SetNull(grantObjectType), diags
		}
		grants = append(grants, grantModel{
			Role:       types.StringValue(b.Role),
			Privileges: privSet,
		})
	}
	return types.SetValueFrom(ctx, grantObjectType, grants)
}

// bindingsFromGrants converts a known `grant` set value into role bindings.
func bindingsFromGrants(ctx context.Context, grantSet types.Set) (bindings []scylladb.AuthoritativeBinding, diags diag.Diagnostics) {
	var grants []grantModel
	diags.Append(grantSet.ElementsAs(ctx, &grants, false)...)
	if diags.HasError() {
		return nil, diags
	}
	bindings = make([]scylladb.AuthoritativeBinding, 0, len(grants))
	for _, g := range grants {
		var privs []string
		diags.Append(g.Privileges.ElementsAs(ctx, &privs, false)...)
		if diags.HasError() {
			return nil, diags
		}
		bindings = append(bindings, scylladb.AuthoritativeBinding{
			Privileges: privs,
			Role:       g.Role.ValueString(),
		})
	}
	return bindings, diags
}
