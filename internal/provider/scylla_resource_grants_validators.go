// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// uniqueGrantRolesValidator rejects a grant set that contains duplicate role values.
type uniqueGrantRolesValidator struct{}

func (uniqueGrantRolesValidator) Description(_ context.Context) string {
	return "Grant roles must be unique across all grant blocks."
}

func (uniqueGrantRolesValidator) MarkdownDescription(_ context.Context) string {
	return "Grant roles must be unique across all grant blocks."
}

func (v uniqueGrantRolesValidator) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}
	var grants []grantModel
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &grants, true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	seen := make(map[string]bool, len(grants))
	for _, g := range grants {
		if g.Role.IsUnknown() || g.Role.IsNull() {
			continue
		}
		role := g.Role.ValueString()
		if seen[role] {
			resp.Diagnostics.AddAttributeError(req.Path, "Duplicate Grant Role",
				fmt.Sprintf("Role %q appears in more than one grant block.", role))
			return
		}
		seen[role] = true
	}
}
