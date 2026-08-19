package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCustomerIDMatchesStatePlanModifier(t *testing.T) {
	ctx := context.Background()
	mod := customerIDMatchesState()

	if mod.Description(ctx) == "" {
		t.Error("expected non-empty description")
	}
	if mod.MarkdownDescription(ctx) == "" {
		t.Error("expected non-empty markdown description")
	}

	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"customer_id": tftypes.String,
			"id":          tftypes.String,
		},
	}

	t.Run("null state (create before import) -> no error", func(t *testing.T) {
		req := planmodifier.StringRequest{
			State: tfsdk.State{
				Raw: tftypes.NewValue(objectType, nil),
			},
			ConfigValue: types.StringValue("cust-123"),
			StateValue:  types.StringNull(),
			Path:        path.Root("customer_id"),
		}
		var resp planmodifier.StringResponse
		mod.PlanModifyString(ctx, req, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no error, got: %s", resp.Diagnostics.Errors()[0].Detail())
		}
	})

	t.Run("null config (omitted) with prior state -> no error", func(t *testing.T) {
		req := planmodifier.StringRequest{
			State: tfsdk.State{
				Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
					"customer_id": tftypes.NewValue(tftypes.String, "cust-123"),
					"id":          tftypes.NewValue(tftypes.String, "cust-123"),
				}),
			},
			ConfigValue: types.StringNull(),
			StateValue:  types.StringValue("cust-123"),
			Path:        path.Root("customer_id"),
		}
		var resp planmodifier.StringResponse
		mod.PlanModifyString(ctx, req, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no error, got: %s", resp.Diagnostics.Errors()[0].Detail())
		}
	})

	t.Run("matching config and state -> no error", func(t *testing.T) {
		req := planmodifier.StringRequest{
			State: tfsdk.State{
				Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
					"customer_id": tftypes.NewValue(tftypes.String, "cust-123"),
					"id":          tftypes.NewValue(tftypes.String, "cust-123"),
				}),
			},
			ConfigValue: types.StringValue("cust-123"),
			StateValue:  types.StringValue("cust-123"),
			Path:        path.Root("customer_id"),
		}
		var resp planmodifier.StringResponse
		mod.PlanModifyString(ctx, req, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no error, got: %s", resp.Diagnostics.Errors()[0].Detail())
		}
	})

	t.Run("mismatched config and state -> returns attribute error", func(t *testing.T) {
		req := planmodifier.StringRequest{
			State: tfsdk.State{
				Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
					"customer_id": tftypes.NewValue(tftypes.String, "cust-123"),
					"id":          tftypes.NewValue(tftypes.String, "cust-123"),
				}),
			},
			ConfigValue: types.StringValue("cust-456"),
			StateValue:  types.StringValue("cust-123"),
			Path:        path.Root("customer_id"),
		}
		var resp planmodifier.StringResponse
		mod.PlanModifyString(ctx, req, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error diagnostic, got none")
		}
		err := resp.Diagnostics.Errors()[0]
		if !strings.Contains(err.Summary(), "Invalid Customer ID") {
			t.Errorf("expected summary to contain 'Invalid Customer ID', got: %s", err.Summary())
		}
		if !strings.Contains(err.Detail(), "cust-456") || !strings.Contains(err.Detail(), "cust-123") {
			t.Errorf("expected detail to mention cust-456 and cust-123, got: %s", err.Detail())
		}
	})
}
