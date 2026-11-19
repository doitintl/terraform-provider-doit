package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestCloudconnectAwsAccountResource_ModifyPlan_ClearOnlyUpdate verifies that an
// update whose only change is a cleared s3bucket and s3bucket_region (planned
// "" by the clearing modifiers, with every other value the stored one) plans
// supported_features unknown.
func TestCloudconnectAwsAccountResource_ModifyPlan_ClearOnlyUpdate(t *testing.T) {
	ctx := t.Context()
	r := &cloudconnectAwsAccountResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	sch := schemaResp.Schema

	objType := sch.Type().TerraformType(ctx).(tftypes.Object)
	featureType := objType.AttributeTypes["supported_features"].(tftypes.List).ElementType
	feature := tftypes.NewValue(featureType, map[string]tftypes.Value{
		"has_required_permissions": tftypes.NewValue(tftypes.Bool, true),
		"name":                     tftypes.NewValue(tftypes.String, "core"),
	})
	object := func(s3bucket, s3bucketRegion string) tftypes.Value {
		values := map[string]tftypes.Value{}
		for name, attrType := range objType.AttributeTypes {
			values[name] = tftypes.NewValue(attrType, nil)
		}
		values["account_id"] = tftypes.NewValue(tftypes.String, "123456789012")
		values["role_arn"] = tftypes.NewValue(tftypes.String, "arn:aws:iam::123456789012:role/doit")
		values["enabled_features"] = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
			tftypes.NewValue(tftypes.String, "core"),
		})
		values["s3bucket"] = tftypes.NewValue(tftypes.String, s3bucket)
		values["s3bucket_region"] = tftypes.NewValue(tftypes.String, s3bucketRegion)
		values["supported_features"] = tftypes.NewValue(objType.AttributeTypes["supported_features"], []tftypes.Value{feature})
		values["time_linked"] = tftypes.NewValue(tftypes.String, "2026-01-01T00:00:00Z")
		return tftypes.NewValue(objType, values)
	}
	state := object("bucket", "us-east-1")
	plan := object("", "")

	req := resource.ModifyPlanRequest{
		State: tfsdk.State{Schema: sch, Raw: state},
		Plan:  tfsdk.Plan{Schema: sch, Raw: plan},
	}
	resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: sch, Raw: plan.Copy()}}
	r.ModifyPlan(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var supportedFeatures types.List
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("supported_features"), &supportedFeatures)...)
	var timeLinked types.String
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("time_linked"), &timeLinked)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if !supportedFeatures.IsUnknown() {
		t.Errorf("expected supported_features to be unknown, got %s", supportedFeatures)
	}
	if timeLinked.ValueString() != "2026-01-01T00:00:00Z" {
		t.Errorf("expected time_linked to keep its stored value, got %s", timeLinked)
	}
}
