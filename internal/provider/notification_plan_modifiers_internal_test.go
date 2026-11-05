package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPlanNotificationMetadataUnknownOnUpdate(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"name":        schema.StringAttribute{Required: true},
		"etag":        schema.StringAttribute{Computed: true},
		"update_time": schema.StringAttribute{Computed: true},
	}}
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "etag": tftypes.String, "update_time": tftypes.String,
	}}
	object := func(name, etag, updateTime any) tftypes.Value {
		return tftypes.NewValue(objType, map[string]tftypes.Value{
			"name":        tftypes.NewValue(tftypes.String, name),
			"etag":        tftypes.NewValue(tftypes.String, etag),
			"update_time": tftypes.NewValue(tftypes.String, updateTime),
		})
	}
	null := tftypes.NewValue(objType, nil)
	stored := object("a", "e1", "t1")

	tests := []struct {
		name        string
		state, plan tftypes.Value
		want        tftypes.Value
	}{
		{"create", null, object("a", tftypes.UnknownValue, tftypes.UnknownValue), object("a", tftypes.UnknownValue, tftypes.UnknownValue)},
		{"destroy", stored, null, null},
		{"no change", stored, object("a", "e1", "t1"), object("a", "e1", "t1")},
		{"update with stored metadata", stored, object("b", "e1", "t1"), object("b", tftypes.UnknownValue, tftypes.UnknownValue)},
		{"update already unknown", stored, object("b", tftypes.UnknownValue, tftypes.UnknownValue), object("b", tftypes.UnknownValue, tftypes.UnknownValue)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				State: tfsdk.State{Schema: s, Raw: tt.state},
				Plan:  tfsdk.Plan{Schema: s, Raw: tt.plan},
			}
			resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: s, Raw: tt.plan.Copy()}}

			planNotificationMetadataUnknownOnUpdate(t.Context(), req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if !resp.Plan.Raw.Equal(tt.want) {
				t.Errorf("plan = %s, want %s", resp.Plan.Raw, tt.want)
			}
		})
	}
}
