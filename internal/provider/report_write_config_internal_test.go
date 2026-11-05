package provider

import (
	"encoding/json"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestToExternalConfigWrite_MatchesExternalConfig guards the conversion behind
// report create and update requests: ExternalConfigWrite must marshal to the same
// JSON as the ExternalConfig that ad-hoc queries send, including explicit nulls on
// the clearable nested attributes and every enum that has its own write copy.
func TestToExternalConfigWrite_MatchesExternalConfig(t *testing.T) {
	ctx := t.Context()

	populated := func() map[string]attr.Value {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		metric := resource_report.NewMetricValueMust(resource_report.MetricValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"type":  types.StringValue("basic"),
			"value": types.StringValue("cost"),
		})

		cfgMap["aggregation"] = types.StringValue("count")
		cfgMap["data_source"] = types.StringValue("billing")
		cfgMap["display_values"] = types.StringValue("actuals_only")
		cfgMap["limit_aggregation"] = types.StringValue("top")
		cfgMap["sort_dimensions"] = types.StringValue("a_to_z")
		cfgMap["sort_groups"] = types.StringValue("desc")
		cfgMap["time_interval"] = types.StringValue("month")
		cfgMap["count"] = resource_report.NewCountValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"id":   types.StringValue("service_description"),
			"type": types.StringValue("fixed"),
		})
		cfgMap["custom_time_range"] = resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"from": types.StringValue("2025-01-01T00:00:00Z"),
			"to":   types.StringValue("2025-02-01T00:00:00Z"),
		})
		cfgMap["limit_by_change"] = resource_report.NewLimitByChangeValueMust(resource_report.LimitByChangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"change_type":             types.StringValue("percentage"),
			"include_incomplete_data": types.BoolValue(false),
			"metric":                  metric,
			"operator":                types.StringValue("gte"),
			"values":                  types.ListValueMust(types.Float64Type, []attr.Value{types.Float64Value(50)}),
		})
		cfgMap["metric_filter"] = resource_report.NewMetricFilterValueMust(resource_report.MetricFilterValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"metric":   metric,
			"operand":  types.StringNull(),
			"operator": types.StringValue("gt"),
			"values":   types.ListValueMust(types.Float64Type, []attr.Value{types.Float64Value(100)}),
		})

		filter := resource_report.NewFiltersValueMust(resource_report.FiltersValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"case_insensitive": types.BoolValue(true),
			"id":               types.StringValue("cloud_provider"),
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolValue(true),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("fixed"),
			"values":           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("google-cloud")}),
		})
		cfgMap["filters"] = types.ListValueMust(resource_report.FiltersValue{}.Type(ctx), []attr.Value{filter})

		limit := resource_report.NewLimitValueMust(resource_report.LimitValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"metric": metric,
			"sort":   types.StringValue("desc"),
			"value":  types.Int64Value(5),
		})
		group := resource_report.NewGroupValueMust(resource_report.GroupValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"id":    types.StringValue("service_description"),
			"limit": limit,
			"type":  types.StringValue("fixed"),
		})
		cfgMap["group"] = types.ListValueMust(resource_report.GroupValue{}.Type(ctx), []attr.Value{group})

		return cfgMap
	}

	tests := []struct {
		name   string
		cfgMap map[string]attr.Value
	}{
		{name: "nullable attributes null", cfgMap: validCumulativeComparisonConfigMap(ctx, t)},
		{name: "everything populated", cfgMap: populated()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), tt.cfgMap)
			if diags.HasError() {
				t.Fatalf("NewConfigValue: %v", diags)
			}

			query, diags := toExternalConfig(ctx, cfgVal)
			if diags.HasError() {
				t.Fatalf("toExternalConfig: %v", diags)
			}
			write, diags := toExternalConfigWrite(ctx, cfgVal)
			if diags.HasError() {
				t.Fatalf("toExternalConfigWrite: %v", diags)
			}

			wantJSON, err := json.Marshal(query)
			if err != nil {
				t.Fatalf("marshal query config: %v", err)
			}
			gotJSON, err := json.Marshal(write)
			if err != nil {
				t.Fatalf("marshal write config: %v", err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("write config JSON differs from query config JSON\n got: %s\nwant: %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestToExternalConfigWrite_NullAndUnknownConfig(t *testing.T) {
	ctx := t.Context()

	for name, cfg := range map[string]resource_report.ConfigValue{
		"null":    resource_report.NewConfigValueNull(),
		"unknown": resource_report.NewConfigValueUnknown(),
	} {
		t.Run(name, func(t *testing.T) {
			write, diags := toExternalConfigWrite(ctx, cfg)
			if diags.HasError() {
				t.Fatalf("toExternalConfigWrite: %v", diags)
			}
			if write != nil {
				t.Errorf("expected nil write config for a %s config, got %+v", name, write)
			}
		})
	}
}

// The API merges an update with the stored report and checks the merged data
// source, and an omitted data source resets to the tenant default instead of
// keeping the stored one. The request therefore has to carry the planned value.
func TestToExternalConfigWrite_SendsPlannedDataSource(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name       string
		dataSource types.String
		want       *string
	}{
		{name: "billing", dataSource: types.StringValue("billing"), want: new("billing")},
		{name: "billing-datahub", dataSource: types.StringValue("billing-datahub"), want: new("billing-datahub")},
		{name: "null is omitted", dataSource: types.StringNull(), want: nil},
		{name: "unknown is omitted", dataSource: types.StringUnknown(), want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMap := validCumulativeComparisonConfigMap(ctx, t)
			cfgMap["data_source"] = tt.dataSource
			cfgVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), cfgMap)
			if diags.HasError() {
				t.Fatalf("NewConfigValue: %v", diags)
			}

			write, diags := toExternalConfigWrite(ctx, cfgVal)
			if diags.HasError() {
				t.Fatalf("toExternalConfigWrite: %v", diags)
			}

			switch {
			case tt.want == nil && write.DataSource != nil:
				t.Errorf("dataSource = %q, want it omitted", *write.DataSource)
			case tt.want != nil && (write.DataSource == nil || string(*write.DataSource) != *tt.want):
				t.Errorf("dataSource = %v, want %q", write.DataSource, *tt.want)
			}
		})
	}
}

// A null plan value is not always serialized the same way. Nullable objects
// (count) send an explicit JSON null so the PATCH merge clears the stored value,
// while pointer fields with omitempty (a filter's mode) are left out of the
// request. The implement-resource skill tells request builders to pick the
// representation per attribute, so this pins both.
func TestToExternalConfigWrite_NullPlanSerialization(t *testing.T) {
	ctx := t.Context()

	cfgMap := validCumulativeComparisonConfigMap(ctx, t)
	filter := resource_report.NewFiltersValueMust(resource_report.FiltersValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"case_insensitive": types.BoolNull(),
		"id":               types.StringValue("cloud_provider"),
		"include_null":     types.BoolNull(),
		"inverse":          types.BoolNull(),
		"mode":             types.StringNull(),
		"type":             types.StringValue("fixed"),
		"values":           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("google-cloud")}),
	})
	cfgMap["filters"] = types.ListValueMust(resource_report.FiltersValue{}.Type(ctx), []attr.Value{filter})

	cfgVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), cfgMap)
	if diags.HasError() {
		t.Fatalf("NewConfigValue: %v", diags)
	}
	write, diags := toExternalConfigWrite(ctx, cfgVal)
	if diags.HasError() {
		t.Fatalf("toExternalConfigWrite: %v", diags)
	}
	gotJSON, err := json.Marshal(write)
	if err != nil {
		t.Fatalf("marshal write config: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatalf("unmarshal write config: %v", err)
	}
	if count, ok := got["count"]; !ok || string(count) != "null" {
		t.Errorf("count: want an explicit JSON null, got %q (present=%v)", count, ok)
	}
	var filters []map[string]json.RawMessage
	if err := json.Unmarshal(got["filters"], &filters); err != nil || len(filters) != 1 {
		t.Fatalf("filters: got %s, err %v", got["filters"], err)
	}
	if mode, ok := filters[0]["mode"]; ok {
		t.Errorf("filter mode: want it omitted from the request, got %s", mode)
	}
}

// An omitted data_source plans unknown on update and the request leaves it out,
// which the API resets to the tenant default. The update request therefore has
// to carry the stored value when the plan has none.
func TestReportModelWithStoredDataSource(t *testing.T) {
	ctx := t.Context()

	model := func(dataSource types.String) reportResourceModel {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["data_source"] = dataSource
		cfg, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), cfgMap)
		if diags.HasError() {
			t.Fatalf("NewConfigValue: %v", diags)
		}
		return reportResourceModel{Config: cfg}
	}

	tests := []struct {
		name  string
		plan  types.String
		state types.String
		want  types.String
	}{
		{name: "unknown plan takes the stored value", plan: types.StringUnknown(), state: types.StringValue("billing"), want: types.StringValue("billing")},
		{name: "known plan wins", plan: types.StringValue("bqlens"), state: types.StringValue("billing"), want: types.StringValue("bqlens")},
		{name: "unknown plan with no stored value stays unknown", plan: types.StringUnknown(), state: types.StringNull(), want: types.StringUnknown()},
		{name: "null plan stays null", plan: types.StringNull(), state: types.StringValue("billing"), want: types.StringNull()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model(tt.plan).withStoredDataSource(model(tt.state))
			if !got.Config.DataSource.Equal(tt.want) {
				t.Errorf("data_source = %s, want %s", got.Config.DataSource, tt.want)
			}
		})
	}
}
