package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_planned_purchases"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func buildPs4cGcpPlannedPurchasesConfig(ctx context.Context, overrides map[string]tftypes.Value) tfsdk.Config {
	schema := datasource_ps4c_gcp_planned_purchases.Ps4cGcpPlannedPurchasesDataSourceSchema(ctx)

	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range schema.Attributes {
		tfType := attribute.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if val, ok := overrides[name]; ok {
			values[name] = val
		} else if name == "billing_account_id" {
			values[name] = tftypes.NewValue(tfType, "012345-6789AB-CDEF01")
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	return tfsdk.Config{
		Schema: schema,
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values),
	}
}

func TestPs4cGcpPlannedPurchasesServiceRegionValidator(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	tests := []struct {
		name        string
		overrides   map[string]tftypes.Value
		expectErr   bool
		wantSummary string
	}{
		{
			name: "valid compute with global",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "compute"),
				"region":      tftypes.NewValue(tftypes.String, "global"),
			},
			expectErr: false,
		},
		{
			name: "valid cloud_sql with concrete region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "cloud_sql"),
				"region":      tftypes.NewValue(tftypes.String, "us_central1"),
			},
			expectErr: false,
		},
		{
			name:      "valid both null",
			overrides: map[string]tftypes.Value{},
			expectErr: false,
		},
		{
			name: "valid compute with null region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "compute"),
			},
			expectErr: false,
		},
		{
			name: "valid cloud_sql with null region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "cloud_sql"),
			},
			expectErr: false,
		},
		{
			name: "invalid region without service (global)",
			overrides: map[string]tftypes.Value{
				"region": tftypes.NewValue(tftypes.String, "global"),
			},
			expectErr:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name: "invalid region without service (concrete)",
			overrides: map[string]tftypes.Value{
				"region": tftypes.NewValue(tftypes.String, "us_central1"),
			},
			expectErr:   true,
			wantSummary: "Invalid Attribute Combination",
		},
		{
			name: "invalid compute with concrete region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "compute"),
				"region":      tftypes.NewValue(tftypes.String, "us_central1"),
			},
			expectErr:   true,
			wantSummary: "Invalid Region for Service",
		},
		{
			name: "invalid cloud_sql with global",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "cloud_sql"),
				"region":      tftypes.NewValue(tftypes.String, "global"),
			},
			expectErr:   true,
			wantSummary: "Invalid Region for Service",
		},
		{
			name: "deferred unknown service with known region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"region":      tftypes.NewValue(tftypes.String, "global"),
			},
			expectErr: false,
		},
		{
			name: "deferred known service with unknown region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, "compute"),
				"region":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
			expectErr: false,
		},
		{
			name: "deferred both service and region unknown",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"region":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
			expectErr: false,
		},
		{
			name: "deferred unknown service with null region",
			overrides: map[string]tftypes.Value{
				"gcp_service": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := buildPs4cGcpPlannedPurchasesConfig(ctx, tt.overrides)
			validator := ps4cGcpPlannedPurchasesServiceRegionValidator{}
			req := datasource.ValidateConfigRequest{Config: cfg}
			var resp datasource.ValidateConfigResponse

			validator.ValidateDataSource(ctx, req, &resp)

			if tt.expectErr {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected error diagnostic, got none")
				}
				if tt.wantSummary != "" {
					found := false
					for _, diag := range resp.Diagnostics.Errors() {
						if diag.Summary() == tt.wantSummary {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error summary %q, got: %v", tt.wantSummary, resp.Diagnostics.Errors())
					}
				}
			} else {
				if resp.Diagnostics.HasError() {
					t.Fatalf("unexpected error diagnostic: %v", resp.Diagnostics)
				}
			}
		})
	}
}
