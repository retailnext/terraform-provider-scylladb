// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

func grantsResourceSchemas(t *testing.T) map[string]schema.Schema {
	t.Helper()
	ctx := context.Background()
	schemas := make(map[string]schema.Schema)
	for name, r := range map[string]resource.Resource{
		"keyspace_grants": NewKeyspaceGrantsResource(),
		"table_grants":    NewTableGrantsResource(),
	} {
		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: Schema: %v", name, resp.Diagnostics)
		}
		schemas[name] = resp.Schema
	}
	return schemas
}

func mustGrantSet(t *testing.T, grants ...grantModel) types.Set {
	t.Helper()
	if grants == nil {
		grants = []grantModel{}
	}
	set, diags := types.SetValueFrom(context.Background(), grantObjectType, grants)
	if diags.HasError() {
		t.Fatalf("SetValueFrom: %v", diags)
	}
	return set
}

func mustGrant(t *testing.T, role string, privs ...string) grantModel {
	t.Helper()
	privSet, diags := types.SetValueFrom(context.Background(), types.StringType, privs)
	if diags.HasError() {
		t.Fatalf("SetValueFrom: %v", diags)
	}
	return grantModel{Role: types.StringValue(role), Privileges: privSet}
}

func TestGrantObjectTypeMatchesSchema(t *testing.T) {
	for name, s := range grantsResourceSchemas(t) {
		block, ok := s.Blocks["grant"]
		if !ok {
			t.Fatalf("%s: schema has no grant block", name)
		}
		setType, ok := block.Type().(types.SetType)
		if !ok {
			t.Fatalf("%s: grant block type is %T, want types.SetType", name, block.Type())
		}
		if !setType.ElemType.Equal(grantObjectType) {
			t.Errorf("%s: grant element type %s does not match grantObjectType %s", name, setType.ElemType, grantObjectType)
		}
	}
}

func TestGetGrantsFromRoleBindings(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		bindings []scylladb.AuthoritativeBinding
		want     types.Set
	}{
		{
			name:     "nil bindings yield empty set",
			bindings: nil,
			want:     mustGrantSet(t),
		},
		{
			name: "multiple roles",
			bindings: []scylladb.AuthoritativeBinding{
				{Role: "alice", Privileges: []string{"SELECT", "MODIFY"}},
				{Role: "bob", Privileges: []string{"ALTER"}},
			},
			want: mustGrantSet(t,
				mustGrant(t, "bob", "ALTER"),
				mustGrant(t, "alice", "MODIFY", "SELECT"),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := GetGrantsFromRoleBindings(ctx, tt.bindings)
			if diags.HasError() {
				t.Fatalf("GetGrantsFromRoleBindings: %v", diags)
			}
			if got.IsNull() || got.IsUnknown() {
				t.Fatalf("got null/unknown set: %s", got)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestBindingsFromGrants(t *testing.T) {
	ctx := context.Background()

	bindings, diags := bindingsFromGrants(ctx, mustGrantSet(t,
		mustGrant(t, "alice", "SELECT", "MODIFY"),
		mustGrant(t, "bob", "ALTER"),
	))
	if diags.HasError() {
		t.Fatalf("bindingsFromGrants: %v", diags)
	}

	got := make(map[string]map[string]bool, len(bindings))
	for _, b := range bindings {
		privs := make(map[string]bool, len(b.Privileges))
		for _, p := range b.Privileges {
			privs[p] = true
		}
		got[b.Role] = privs
	}
	want := map[string]map[string]bool{
		"alice": {"SELECT": true, "MODIFY": true},
		"bob":   {"ALTER": true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for role, wantPrivs := range want {
		gotPrivs := got[role]
		if len(gotPrivs) != len(wantPrivs) {
			t.Errorf("role %s: got %v, want %v", role, gotPrivs, wantPrivs)
			continue
		}
		for p := range wantPrivs {
			if !gotPrivs[p] {
				t.Errorf("role %s: missing privilege %s", role, p)
			}
		}
	}
}

// A `dynamic "grant"` block whose for_each is not yet known makes the whole
// grant set unknown; the resource models must still decode it.
func TestGrantsResourceModelsDecodeUnknownGrants(t *testing.T) {
	ctx := context.Background()
	schemas := grantsResourceSchemas(t)

	decode := map[string]func(tfsdk.Plan) (types.Set, diag.Diagnostics){
		"keyspace_grants": func(p tfsdk.Plan) (types.Set, diag.Diagnostics) {
			var m keyspaceGrantsResourceModel
			diags := p.Get(ctx, &m)
			return m.Grants, diags
		},
		"table_grants": func(p tfsdk.Plan) (types.Set, diag.Diagnostics) {
			var m tableGrantsResourceModel
			diags := p.Get(ctx, &m)
			return m.Grants, diags
		},
	}

	for name, s := range schemas {
		t.Run(name, func(t *testing.T) {
			objType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
			if !ok {
				t.Fatalf("schema type is %T, want tftypes.Object", s.Type().TerraformType(ctx))
			}
			vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
			for attrName, attrType := range objType.AttributeTypes {
				vals[attrName] = tftypes.NewValue(attrType, nil)
			}
			vals["keyspace"] = tftypes.NewValue(tftypes.String, "ks")
			vals["grant"] = tftypes.NewValue(objType.AttributeTypes["grant"], tftypes.UnknownValue)

			plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(objType, vals)}
			grants, diags := decode[name](plan)
			if diags.HasError() {
				t.Fatalf("Plan.Get: %v", diags)
			}
			if !grants.IsUnknown() {
				t.Errorf("got %s, want unknown", grants)
			}
		})
	}
}
