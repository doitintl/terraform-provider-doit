package provider

import (
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPlanVolatileAttributesUnknownOnUpdate(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"name":        schema.StringAttribute{Required: true},
		"etag":        schema.StringAttribute{Computed: true},
		"update_time": schema.StringAttribute{Computed: true},
		"modified":    schema.Int64Attribute{Computed: true},
		"utilization": schema.Float64Attribute{Computed: true},
	}}
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "etag": tftypes.String, "update_time": tftypes.String,
		"modified": tftypes.Number, "utilization": tftypes.Number,
	}}
	unknown := tftypes.UnknownValue
	object := func(name, etag, updateTime, modified, utilization any) tftypes.Value {
		return tftypes.NewValue(objType, map[string]tftypes.Value{
			"name":        tftypes.NewValue(tftypes.String, name),
			"etag":        tftypes.NewValue(tftypes.String, etag),
			"update_time": tftypes.NewValue(tftypes.String, updateTime),
			"modified":    tftypes.NewValue(tftypes.Number, modified),
			"utilization": tftypes.NewValue(tftypes.Number, utilization),
		})
	}
	null := tftypes.NewValue(objType, nil)
	stored := object("a", "e1", "t1", big.NewFloat(1700), big.NewFloat(0.5))

	tests := []struct {
		name        string
		state, plan tftypes.Value
		want        tftypes.Value
	}{
		{"create", null, object("a", unknown, unknown, unknown, unknown), object("a", unknown, unknown, unknown, unknown)},
		{"destroy", stored, null, null},
		{"no change", stored, object("a", "e1", "t1", big.NewFloat(1700), big.NewFloat(0.5)), object("a", "e1", "t1", big.NewFloat(1700), big.NewFloat(0.5))},
		{"update with stored metadata", stored, object("b", "e1", "t1", big.NewFloat(1700), big.NewFloat(0.5)), object("b", unknown, unknown, unknown, unknown)},
		{"update already unknown", stored, object("b", unknown, unknown, unknown, unknown), object("b", unknown, unknown, unknown, unknown)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				State: tfsdk.State{Schema: s, Raw: tt.state},
				Plan:  tfsdk.Plan{Schema: s, Raw: tt.plan},
			}
			resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: s, Raw: tt.plan.Copy()}}

			planVolatileAttributesUnknownOnUpdate(t.Context(), req, resp, path.Root("etag"), path.Root("update_time"), path.Root("modified"), path.Root("utilization"))

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if !resp.Plan.Raw.Equal(tt.want) {
				t.Errorf("plan = %s, want %s", resp.Plan.Raw, tt.want)
			}
		})
	}
}
