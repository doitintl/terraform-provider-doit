package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendations"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func buildPs4cGcpRecommendationValidatorConfig(ctx context.Context, t *testing.T, values map[string]attr.Value) tfsdk.Config {
	t.Helper()

	recommendationSchema := datasource_ps4c_gcp_recommendation.Ps4cGcpRecommendationDataSourceSchema(ctx)
	schemaType := recommendationSchema.Type().TerraformType(ctx)
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
		Schema: recommendationSchema,
		Raw:    tftypes.NewValue(schemaType, terraformValues),
	}
}

func buildPs4cGcpRecommendationsValidatorConfig(ctx context.Context, t *testing.T, values map[string]attr.Value) tfsdk.Config {
	t.Helper()

	recommendationsSchema := datasource_ps4c_gcp_recommendations.Ps4cGcpRecommendationsDataSourceSchema(ctx)
	schemaType := recommendationsSchema.Type().TerraformType(ctx)
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
		Schema: recommendationsSchema,
		Raw:    tftypes.NewValue(schemaType, terraformValues),
	}
}

func TestPs4cGcpRecommendationConfigValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		gcpService  types.String
		region      types.String
		wantError   bool
		wantSummary string
	}{
		{
			name:       "both unknown is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:       "service unknown with region global is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringValue("global"),
			wantError:  false,
		},
		{
			name:       "service unknown with concrete region is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringValue("us-central1"),
			wantError:  false,
		},
		{
			name:       "service unknown with region null is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringNull(),
			wantError:  false,
		},
		{
			name:       "compute with unknown region is deferred",
			gcpService: types.StringValue("compute"),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:       "compute with null region passes (defaults to global)",
			gcpService: types.StringValue("compute"),
			region:     types.StringNull(),
			wantError:  false,
		},
		{
			name:       "compute with global region passes",
			gcpService: types.StringValue("compute"),
			region:     types.StringValue("global"),
			wantError:  false,
		},
		{
			name:        "compute with concrete region kebab-case fails",
			gcpService:  types.StringValue("compute"),
			region:      types.StringValue("us-central1"),
			wantError:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name:        "compute with concrete region snake_case fails",
			gcpService:  types.StringValue("compute"),
			region:      types.StringValue("us_central1"),
			wantError:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name:       "cloud_sql with unknown region is deferred",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:        "cloud_sql with null region fails",
			gcpService:  types.StringValue("cloud_sql"),
			region:      types.StringNull(),
			wantError:   true,
			wantSummary: "Missing Required Attribute",
		},
		{
			name:        "cloud_sql with global region fails",
			gcpService:  types.StringValue("cloud_sql"),
			region:      types.StringValue("global"),
			wantError:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name:       "cloud_sql with concrete region kebab-case passes",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringValue("us-central1"),
			wantError:  false,
		},
		{
			name:       "cloud_sql with concrete region snake_case passes",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringValue("us_central1"),
			wantError:  false,
		},
		{
			name:       "service null with region null is deferred/skipped by validator",
			gcpService: types.StringNull(),
			region:     types.StringNull(),
			wantError:  false,
		},
	}

	ctx := t.Context()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildPs4cGcpRecommendationValidatorConfig(ctx, t, map[string]attr.Value{
				"billing_account_id": types.StringValue("012345-6789AB-CDEF01"),
				"gcp_service":        tt.gcpService,
				"region":             tt.region,
			})
			resp := &datasource.ValidateConfigResponse{}
			ps4cGcpRecommendationConfigValidator{}.ValidateDataSource(ctx, datasource.ValidateConfigRequest{Config: config}, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diagnostics: %v)", got, tt.wantError, resp.Diagnostics)
			}
			errorCount := 0
			for _, diagnostic := range resp.Diagnostics {
				if diagnostic.Severity() != diag.SeverityError {
					continue
				}
				errorCount++
				if diagnostic.Summary() != tt.wantSummary {
					t.Errorf("error summary = %q, want %q", diagnostic.Summary(), tt.wantSummary)
				}
				withPath, ok := diagnostic.(diag.DiagnosticWithPath)
				if !ok {
					t.Errorf("expected diagnostic with path, got %T", diagnostic)
				} else if !withPath.Path().Equal(path.Root("region")) {
					t.Errorf("error path = %v, want %v", withPath.Path(), path.Root("region"))
				}
			}
			wantErrorCount := 0
			if tt.wantError {
				wantErrorCount = 1
			}
			if errorCount != wantErrorCount {
				t.Errorf("error count = %d, want %d (diagnostics: %v)", errorCount, wantErrorCount, resp.Diagnostics)
			}
		})
	}
}

func TestPs4cGcpRecommendationsConfigValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		gcpService  types.String
		region      types.String
		wantError   bool
		wantSummary string
	}{
		{
			name:       "both unknown is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:       "service unknown with concrete region is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringValue("us-central1"),
			wantError:  false,
		},
		{
			name:       "service unknown with global region is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringValue("global"),
			wantError:  false,
		},
		{
			name:       "service unknown with null region is deferred",
			gcpService: types.StringUnknown(),
			region:     types.StringNull(),
			wantError:  false,
		},
		{
			name:       "cloud_sql with unknown region is deferred",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:       "compute with unknown region is deferred",
			gcpService: types.StringValue("compute"),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:       "null service with unknown region is deferred",
			gcpService: types.StringNull(),
			region:     types.StringUnknown(),
			wantError:  false,
		},
		{
			name:       "both null (all services and regions) passes",
			gcpService: types.StringNull(),
			region:     types.StringNull(),
			wantError:  false,
		},
		{
			name:       "compute with null region passes (all compute regions)",
			gcpService: types.StringValue("compute"),
			region:     types.StringNull(),
			wantError:  false,
		},
		{
			name:       "cloud_sql with null region passes (all cloud_sql regions)",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringNull(),
			wantError:  false,
		},
		{
			name:        "region concrete without service fails",
			gcpService:  types.StringNull(),
			region:      types.StringValue("us-central1"),
			wantError:   true,
			wantSummary: "Missing Required Attribute",
		},
		{
			name:        "region global without service fails",
			gcpService:  types.StringNull(),
			region:      types.StringValue("global"),
			wantError:   true,
			wantSummary: "Missing Required Attribute",
		},
		{
			name:       "compute with global region passes",
			gcpService: types.StringValue("compute"),
			region:     types.StringValue("global"),
			wantError:  false,
		},
		{
			name:        "compute with concrete region fails",
			gcpService:  types.StringValue("compute"),
			region:      types.StringValue("us-central1"),
			wantError:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name:        "cloud_sql with global region fails",
			gcpService:  types.StringValue("cloud_sql"),
			region:      types.StringValue("global"),
			wantError:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name:       "cloud_sql with concrete region kebab-case passes",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringValue("us-central1"),
			wantError:  false,
		},
		{
			name:       "cloud_sql with concrete region snake_case passes",
			gcpService: types.StringValue("cloud_sql"),
			region:     types.StringValue("us_central1"),
			wantError:  false,
		},
	}

	ctx := t.Context()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildPs4cGcpRecommendationsValidatorConfig(ctx, t, map[string]attr.Value{
				"billing_account_id": types.StringValue("012345-6789AB-CDEF01"),
				"gcp_service":        tt.gcpService,
				"region":             tt.region,
			})
			resp := &datasource.ValidateConfigResponse{}
			ps4cGcpRecommendationsConfigValidator{}.ValidateDataSource(ctx, datasource.ValidateConfigRequest{Config: config}, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Fatalf("HasError() = %v, want %v (diagnostics: %v)", got, tt.wantError, resp.Diagnostics)
			}
			errorCount := 0
			for _, diagnostic := range resp.Diagnostics {
				if diagnostic.Severity() != diag.SeverityError {
					continue
				}
				errorCount++
				if diagnostic.Summary() != tt.wantSummary {
					t.Errorf("error summary = %q, want %q", diagnostic.Summary(), tt.wantSummary)
				}
				withPath, ok := diagnostic.(diag.DiagnosticWithPath)
				if !ok {
					t.Errorf("expected diagnostic with path, got %T", diagnostic)
				} else if !withPath.Path().Equal(path.Root("region")) {
					t.Errorf("error path = %v, want %v", withPath.Path(), path.Root("region"))
				}
			}
			wantErrorCount := 0
			if tt.wantError {
				wantErrorCount = 1
			}
			if errorCount != wantErrorCount {
				t.Errorf("error count = %d, want %d (diagnostics: %v)", errorCount, wantErrorCount, resp.Diagnostics)
			}
		})
	}
}

func TestPs4cGcpRecommendationDataSource_ConfigValidatorsWiring(t *testing.T) {
	t.Parallel()
	ds, ok := NewPs4cGcpRecommendationDataSource().(datasource.DataSourceWithConfigValidators)
	if !ok {
		t.Fatalf("expected DataSourceWithConfigValidators, got %T", ds)
	}
	validators := ds.ConfigValidators(t.Context())
	if len(validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(validators))
	}
	if _, ok := validators[0].(ps4cGcpRecommendationConfigValidator); !ok {
		t.Fatalf("expected ps4cGcpRecommendationConfigValidator, got %T", validators[0])
	}
}

func TestPs4cGcpRecommendationsDataSource_ConfigValidatorsWiring(t *testing.T) {
	t.Parallel()
	ds, ok := NewPs4cGcpRecommendationsDataSource().(datasource.DataSourceWithConfigValidators)
	if !ok {
		t.Fatalf("expected DataSourceWithConfigValidators, got %T", ds)
	}
	validators := ds.ConfigValidators(t.Context())
	if len(validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(validators))
	}
	if _, ok := validators[0].(ps4cGcpRecommendationsConfigValidator); !ok {
		t.Fatalf("expected ps4cGcpRecommendationsConfigValidator, got %T", validators[0])
	}
}
