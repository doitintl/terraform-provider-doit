package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// newTestAlertPlanWithDataSource returns an alert plan whose config.data_source
// is the given value; every other config attribute is a fixed, valid value.
func newTestAlertPlanWithDataSource(t *testing.T, dataSource types.String) *alertResourceModel {
	t.Helper()
	ctx := t.Context()

	metricVal, d := resource_alert.NewMetricValue(
		resource_alert.MetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"type":  types.StringValue("basic"),
			"value": types.StringValue("cost"),
		},
	)
	if d.HasError() {
		t.Fatalf("NewMetricValue: %v", d)
	}
	emptyScopes, d := types.ListValueFrom(ctx, resource_alert.ScopesValue{}.Type(ctx), []resource_alert.ScopesValue{})
	if d.HasError() {
		t.Fatalf("emptyScopes: %v", d)
	}
	configVal, d := resource_alert.NewConfigValue(
		resource_alert.ConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"condition":           types.StringValue("value"),
			"currency":            types.StringNull(),
			"data_source":         dataSource,
			"evaluate_for_each":   types.StringNull(),
			"ignore_values_range": resource_alert.NewIgnoreValuesRangeValueNull(),
			"metric":              metricVal,
			"operator":            types.StringValue("gt"),
			"scopes":              emptyScopes,
			"time_interval":       types.StringValue("month"),
			"value":               types.Float64Value(10.0),
		},
	)
	if d.HasError() {
		t.Fatalf("NewConfigValue: %v", d)
	}

	return &alertResourceModel{
		Name:                    types.StringValue("Test Alert"),
		Recipients:              types.ListNull(types.StringType),
		RecipientsSlackChannels: types.ListNull(resource_alert.RecipientsSlackChannelsValue{}.Type(ctx)),
		Config:                  configVal,
	}
}

// With no plan-time default, an omitted data_source is Unknown on create and
// must not reach the API as "" so the API can resolve its DataHub-aware default.
func TestAlertRequests_DataSource(t *testing.T) {
	tests := []struct {
		name       string
		dataSource types.String
		want       *string
	}{
		{"null is omitted", types.StringNull(), nil},
		{"unknown is omitted", types.StringUnknown(), nil},
		{"billing is sent", types.StringValue("billing"), new("billing")},
		{"billing-datahub is sent", types.StringValue("billing-datahub"), new("billing-datahub")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := newTestAlertPlanWithDataSource(t, tc.dataSource)

			createReq, diags := plan.toAlertRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("toAlertRequest: %v", diags)
			}
			var gotCreate *string
			if createReq.Config.DataSource != nil {
				gotCreate = new(string(*createReq.Config.DataSource))
			}
			assertStringPtr(t, "create", gotCreate, tc.want)

			updateReq, diags := plan.toAlertUpdateRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("toAlertUpdateRequest: %v", diags)
			}
			var gotUpdate *string
			if updateReq.Config.DataSource != nil {
				gotUpdate = new(string(*updateReq.Config.DataSource))
			}
			assertStringPtr(t, "update", gotUpdate, tc.want)
		})
	}
}

func TestOverlayAlertConfig_DataSource(t *testing.T) {
	resolved := newTestAlertPlanWithDataSource(t, types.StringValue("billing-datahub")).Config

	t.Run("unknown resolves from API response", func(t *testing.T) {
		plan := newTestAlertPlanWithDataSource(t, types.StringUnknown()).Config
		if diags := overlayAlertConfig(t.Context(), &resolved, &plan); diags.HasError() {
			t.Fatalf("overlayAlertConfig: %v", diags)
		}
		if got := plan.DataSource.ValueString(); got != "billing-datahub" {
			t.Errorf("data_source = %q, want billing-datahub", got)
		}
	})

	t.Run("known plan value wins", func(t *testing.T) {
		plan := newTestAlertPlanWithDataSource(t, types.StringValue("billing")).Config
		if diags := overlayAlertConfig(t.Context(), &resolved, &plan); diags.HasError() {
			t.Fatalf("overlayAlertConfig: %v", diags)
		}
		if got := plan.DataSource.ValueString(); got != "billing" {
			t.Errorf("data_source = %q, want billing", got)
		}
	})
}

func TestMapAlertToModel_DataSource(t *testing.T) {
	plan := newTestAlertPlanWithDataSource(t, types.StringUnknown())
	resp := &models.Alert{
		Id:   new("alert-1"),
		Name: "Test Alert",
		Config: &models.AlertConfig{
			DataSource:   new(models.CloudAnalyticsAlertConfigDataSourceBillingDatahub),
			Metric:       models.MetricConfig{Type: "basic", Value: "cost"},
			Operator:     models.MetricFilterText("gt"),
			TimeInterval: models.AlertConfigTimeInterval("month"),
			Value:        10,
		},
	}

	if diags := mapAlertToModel(t.Context(), resp, plan); diags.HasError() {
		t.Fatalf("mapAlertToModel: %v", diags)
	}
	if got := plan.Config.DataSource.ValueString(); got != "billing-datahub" {
		t.Errorf("data_source = %q, want billing-datahub", got)
	}
}

func assertStringPtr(t *testing.T, label string, got, want *string) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s data_source = %v, want %v", label, got, want)
	case *got != *want:
		t.Errorf("%s data_source = %q, want %q", label, *got, *want)
	}
}
