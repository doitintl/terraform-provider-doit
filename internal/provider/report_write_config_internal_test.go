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
