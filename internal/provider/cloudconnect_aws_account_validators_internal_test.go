package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_aws_account"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func buildCloudconnectAwsValidatorConfig(ctx context.Context, t *testing.T, values map[string]attr.Value) tfsdk.Config {
	t.Helper()

	cloudconnectSchema := resource_cloudconnect_aws_account.CloudconnectAwsAccountResourceSchema(ctx)
	schemaType := cloudconnectSchema.Type().TerraformType(ctx)
	objectType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}

	terraformValues := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		terraformValues[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range values {
		terraformValue, err := value.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("convert %s to Terraform value: %v", name, err)
		}
		terraformValues[name] = terraformValue
	}

	return tfsdk.Config{
		Schema: cloudconnectSchema,
		Raw:    tftypes.NewValue(schemaType, terraformValues),
	}
}

func TestCloudconnectAwsS3RealTimeValidator(t *testing.T) {
	features := func(values ...attr.Value) types.List {
		return types.ListValueMust(types.StringType, values)
	}

	noRealTime := features(types.StringValue("billing"))
	realTime := features(types.StringValue("real-time-data"))
	unknownFeatures := types.ListUnknown(types.StringType)
	unknownFeatureElement := features(types.StringUnknown())

	tests := []struct {
		name            string
		enabledFeatures types.List
		s3bucket        types.String
		s3bucketRegion  types.String
		wantError       bool
	}{
		{name: "no real-time with unknown S3 fields is deferred", enabledFeatures: noRealTime, s3bucket: types.StringUnknown(), s3bucketRegion: types.StringUnknown()},
		{name: "no real-time with known bucket fails", enabledFeatures: noRealTime, s3bucket: types.StringValue("bucket"), s3bucketRegion: types.StringUnknown(), wantError: true},
		{name: "no real-time with known region fails", enabledFeatures: noRealTime, s3bucket: types.StringUnknown(), s3bucketRegion: types.StringValue("us-east-1"), wantError: true},
		{name: "real-time with unknown S3 fields is deferred", enabledFeatures: realTime, s3bucket: types.StringUnknown(), s3bucketRegion: types.StringUnknown()},
		{name: "real-time with known-null bucket fails", enabledFeatures: realTime, s3bucket: types.StringNull(), s3bucketRegion: types.StringUnknown(), wantError: true},
		{name: "real-time with known-null region fails", enabledFeatures: realTime, s3bucket: types.StringUnknown(), s3bucketRegion: types.StringNull(), wantError: true},
		{name: "unknown feature list is deferred", enabledFeatures: unknownFeatures, s3bucket: types.StringNull(), s3bucketRegion: types.StringNull()},
		{name: "unknown feature element is deferred", enabledFeatures: unknownFeatureElement, s3bucket: types.StringNull(), s3bucketRegion: types.StringNull()},
		{name: "known real-time configuration passes", enabledFeatures: realTime, s3bucket: types.StringValue("bucket"), s3bucketRegion: types.StringValue("us-east-1")},
		{name: "known real-time configuration missing both fields fails", enabledFeatures: realTime, s3bucket: types.StringNull(), s3bucketRegion: types.StringNull(), wantError: true},
		{name: "known non-real-time configuration without S3 passes", enabledFeatures: noRealTime, s3bucket: types.StringNull(), s3bucketRegion: types.StringNull()},
		{name: "known non-real-time configuration with S3 fails", enabledFeatures: noRealTime, s3bucket: types.StringValue("bucket"), s3bucketRegion: types.StringValue("us-east-1"), wantError: true},
	}

	ctx := t.Context()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildCloudconnectAwsValidatorConfig(ctx, t, map[string]attr.Value{
				"enabled_features": tt.enabledFeatures,
				"s3bucket":         tt.s3bucket,
				"s3bucket_region":  tt.s3bucketRegion,
			})
			resp := &resource.ValidateConfigResponse{}
			cloudconnectAwsS3RealTimeValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diagnostics: %v)", got, tt.wantError, resp.Diagnostics)
			}
		})
	}
}
