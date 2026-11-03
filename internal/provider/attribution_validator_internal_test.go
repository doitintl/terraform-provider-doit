package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUnsupportedAttributionTierTypeValidator(t *testing.T) {
	t.Parallel()

	testPath := path.Root("config").AtName("scopes").AtListIndex(0).AtName("type")
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
		{name: "ordinary dimension type", value: types.StringValue("fixed")},
		{name: "deprecated attribution", value: types.StringValue("attribution")},
		{name: "deprecated attribution group", value: types.StringValue("attribution_group")},
		{name: "allocation rule", value: types.StringValue("allocation_rule")},
		{name: "case-sensitive match", value: types.StringValue("Attribution_Tier_1")},
		{name: "tier 1", value: types.StringValue("attribution_tier_1"), wantError: true},
		{name: "tier 2", value: types.StringValue("attribution_tier_2"), wantError: true},
		{name: "tier 3", value: types.StringValue("attribution_tier_3"), wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := &validator.StringResponse{}
			unsupportedAttributionTierTypeValidator{typeName: "doit_alert"}.ValidateString(t.Context(), validator.StringRequest{
				Path:        testPath,
				ConfigValue: test.value,
			}, response)

			wantErrors := 0
			if test.wantError {
				wantErrors = 1
			}
			if got := len(response.Diagnostics); got != wantErrors {
				t.Fatalf("diagnostics = %d, want %d: %v", got, wantErrors, response.Diagnostics)
			}
			if !test.wantError {
				return
			}

			assertAttributeError(t, response.Diagnostics[0], unsupportedAttributionTierTypeSummary, testPath)
			detail := response.Diagnostics[0].Detail()
			for _, want := range []string{test.value.ValueString(), "doit_alert", "doit_report_query", "allocation_rule"} {
				if !strings.Contains(detail, want) {
					t.Errorf("detail %q does not mention %q", detail, want)
				}
			}
		})
	}
}

func TestAttributionReportDataSourceValidator(t *testing.T) {
	t.Parallel()

	testPath := path.Root("config").AtName("data_source")
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
		{name: "billing", value: types.StringValue("billing")},
		{name: "billing-datahub", value: types.StringValue("billing-datahub")},
		{name: "attribution", value: types.StringValue("attribution"), wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := &validator.StringResponse{}
			attributionReportDataSourceValidator{}.ValidateString(t.Context(), validator.StringRequest{
				Path:        testPath,
				ConfigValue: test.value,
			}, response)

			wantErrors := 0
			if test.wantError {
				wantErrors = 1
			}
			if got := len(response.Diagnostics); got != wantErrors {
				t.Fatalf("diagnostics = %d, want %d: %v", got, wantErrors, response.Diagnostics)
			}
			if test.wantError {
				assertAttributeError(t, response.Diagnostics[0], attributionReportReadOnlySummary, testPath)
			}
		})
	}
}

func TestAttributionRejectionSchemaWiring(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	resources := map[string]struct {
		schemaPaths  []string
		typeName     string
		rejectSource bool
	}{
		"report": {schemaPaths: reportDimensionsTypeAttributePaths, typeName: "doit_report", rejectSource: true},
		"alert":  {schemaPaths: []string{"config.scopes[*].type"}, typeName: "doit_alert"},
		"budget": {schemaPaths: []string{"scopes[*].type"}, typeName: "doit_budget"},
	}
	schemas := map[string]resourceWithSchema{
		"report": &reportResource{},
		"alert":  &alertResource{},
		"budget": &budgetResource{},
	}

	for name, test := range resources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := resourceSchemaForTest(ctx, schemas[name])
			for _, attributePath := range test.schemaPaths {
				attribute := resourceStringAttributeAtPath(t, s.Attributes, attributePath)
				count := 0
				for _, candidate := range attribute.Validators {
					if tier, ok := candidate.(unsupportedAttributionTierTypeValidator); ok {
						count++
						if tier.typeName != test.typeName {
							t.Errorf("%s: typeName = %q, want %q", attributePath, tier.typeName, test.typeName)
						}
					}
				}
				if count != 1 {
					t.Errorf("%s: tier validator count = %d, want 1", attributePath, count)
				}
				if !strings.Contains(attribute.MarkdownDescription, attributionTierUnsupportedNote) {
					t.Errorf("%s: markdown description lacks the tier note", attributePath)
				}
				if !strings.Contains(attribute.Description, attributionTierUnsupportedPlainNote) {
					t.Errorf("%s: description lacks the tier note", attributePath)
				}
			}

			if !test.rejectSource {
				return
			}
			attribute := resourceStringAttributeAtPath(t, s.Attributes, "config.data_source")
			if got := countValidators[attributionReportDataSourceValidator](attribute.Validators); got != 1 {
				t.Errorf("config.data_source: read-only validator count = %d, want 1", got)
			}
			if !strings.Contains(attribute.MarkdownDescription, attributionDataSourceReadOnlyNote) {
				t.Error("config.data_source: markdown description lacks the read-only note")
			}
		})
	}
}

// doit_report_query is the one place attribution queries work: it derives its
// config from a fresh copy of the report resource schema, so the resource's
// rejections must not leak into it.
func TestAttributionRejectionNotWiredForReportQuery(t *testing.T) {
	t.Parallel()

	var response datasource.SchemaResponse
	(&reportQueryDataSource{}).Schema(t.Context(), datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("building report query schema: %v", response.Diagnostics.Errors())
	}

	for _, attributePath := range reportDimensionsTypeAttributePaths {
		attribute := dataSourceStringAttributeAtPath(t, response.Schema.Attributes, attributePath)
		if got := countValidators[unsupportedAttributionTierTypeValidator](attribute.Validators); got != 0 {
			t.Errorf("%s: tier validator count = %d, want 0", attributePath, got)
		}
		assertHasDeprecatedDimensionsTypeValidator(t, attribute.Validators)
		if strings.Contains(attribute.MarkdownDescription, attributionTierUnsupportedNote) {
			t.Errorf("%s: carries the resource-only rejection note", attributePath)
		}
		if !strings.Contains(attribute.MarkdownDescription, attributionQueryTierTypeNote) {
			t.Errorf("%s: markdown description lacks the query tier note", attributePath)
		}
	}

	attribute := dataSourceStringAttributeAtPath(t, response.Schema.Attributes, "config.data_source")
	if got := countValidators[attributionReportDataSourceValidator](attribute.Validators); got != 0 {
		t.Errorf("config.data_source: read-only validator count = %d, want 0", got)
	}
	if !strings.Contains(attribute.MarkdownDescription, attributionQueryDataSourceNote) {
		t.Error("config.data_source: markdown description lacks the query note")
	}

	found := false
	for _, configValidator := range (&reportQueryDataSource{}).ConfigValidators(t.Context()) {
		if _, ok := configValidator.(reportAttributionQueryDataSourceValidator); ok {
			found = true
		}
	}
	if !found {
		t.Error("report query does not register reportAttributionQueryDataSourceValidator")
	}
}

// Whether the single-dimension lookup resolves tier types is unverified, so
// doit_dimension documents them but must not reject them.
func TestAttributionTierNotRejectedForDimensionDataSource(t *testing.T) {
	t.Parallel()

	var response datasource.SchemaResponse
	(&dimensionDataSource{}).Schema(t.Context(), datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("building dimension schema: %v", response.Diagnostics.Errors())
	}

	attribute, ok := response.Schema.Attributes["type"].(datasourceschema.StringAttribute)
	if !ok {
		t.Fatalf("type has type %T, want schema.StringAttribute", response.Schema.Attributes["type"])
	}
	if got := countValidators[unsupportedAttributionTierTypeValidator](attribute.Validators); got != 0 {
		t.Errorf("tier validator count = %d, want 0", got)
	}
	if !strings.Contains(attribute.MarkdownDescription, attributionTierDimensionNote) {
		t.Error("markdown description lacks the tier note")
	}
}

func countValidators[V validator.String](validators []validator.String) int {
	count := 0
	for _, candidate := range validators {
		if _, ok := candidate.(V); ok {
			count++
		}
	}
	return count
}

func assertAttributeError(t *testing.T, diagnostic diag.Diagnostic, summary string, wantPath path.Path) {
	t.Helper()

	if got := diagnostic.Severity(); got != diag.SeverityError {
		t.Errorf("severity = %v, want %v", got, diag.SeverityError)
	}
	if got := diagnostic.Summary(); got != summary {
		t.Errorf("summary = %q, want %q", got, summary)
	}
	withPath, ok := diagnostic.(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("diagnostic is not associated with an attribute path: %T", diagnostic)
	}
	if got := withPath.Path(); !got.Equal(wantPath) {
		t.Errorf("path = %s, want %s", got, wantPath)
	}
}

// --- validateReportAttributionQuery ---

// fakeAttributeGetter serves GetAttribute from a map keyed by path string.
type fakeAttributeGetter map[string]attr.Value

func (g fakeAttributeGetter) GetAttribute(_ context.Context, p path.Path, target any) diag.Diagnostics {
	value, ok := g[p.String()]
	if !ok {
		var diags diag.Diagnostics
		diags.AddError("missing test value", p.String())
		return diags
	}
	reflect.ValueOf(target).Elem().Set(reflect.ValueOf(value))
	return nil
}

// nullAttributeValues returns a null value for every attribute in attrTypes,
// for building generated nested values that only set a few fields.
func nullAttributeValues(ctx context.Context, t *testing.T, attrTypes map[string]attr.Type) map[string]attr.Value {
	t.Helper()

	values := make(map[string]attr.Value, len(attrTypes))
	for name, attrType := range attrTypes {
		value, err := attrType.ValueFromTerraform(ctx, tftypes.NewValue(attrType.TerraformType(ctx), nil))
		if err != nil {
			t.Fatalf("null value for %q: %v", name, err)
		}
		values[name] = value
	}
	return values
}

type attributionDim struct {
	id      attr.Value
	dimType attr.Value
	unknown bool // the whole element is unknown
}

func tierDim(id string) attributionDim {
	return attributionDim{id: types.StringValue(id), dimType: types.StringValue("attribution_tier_1")}
}

func plainDim(id, dimType string) attributionDim {
	return attributionDim{id: types.StringValue(id), dimType: types.StringValue(dimType)}
}

func attributionGroupList(ctx context.Context, t *testing.T, dims ...attributionDim) types.List {
	t.Helper()
	attrTypes := resource_report.GroupValue{}.AttributeTypes(ctx)
	values := make([]resource_report.GroupValue, 0, len(dims))
	for _, dim := range dims {
		if dim.unknown {
			values = append(values, resource_report.NewGroupValueUnknown())
			continue
		}
		attributes := nullAttributeValues(ctx, t, attrTypes)
		attributes["id"], attributes["type"] = dim.id, dim.dimType
		values = append(values, resource_report.NewGroupValueMust(attrTypes, attributes))
	}
	list, diags := types.ListValueFrom(ctx, resource_report.GroupValue{}.Type(ctx), values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom group: %v", diags)
	}
	return list
}

func attributionDimensionsList(ctx context.Context, t *testing.T, dims ...attributionDim) types.List {
	t.Helper()
	attrTypes := resource_report.DimensionsValue{}.AttributeTypes(ctx)
	values := make([]resource_report.DimensionsValue, 0, len(dims))
	for _, dim := range dims {
		attributes := nullAttributeValues(ctx, t, attrTypes)
		attributes["id"], attributes["type"] = dim.id, dim.dimType
		values = append(values, resource_report.NewDimensionsValueMust(attrTypes, attributes))
	}
	list, diags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom dimensions: %v", diags)
	}
	return list
}

func attributionFiltersList(ctx context.Context, t *testing.T, dims ...attributionDim) types.List {
	t.Helper()
	attrTypes := resource_report.FiltersValue{}.AttributeTypes(ctx)
	values := make([]resource_report.FiltersValue, 0, len(dims))
	for _, dim := range dims {
		attributes := nullAttributeValues(ctx, t, attrTypes)
		attributes["id"], attributes["type"] = dim.id, dim.dimType
		values = append(values, resource_report.NewFiltersValueMust(attrTypes, attributes))
	}
	list, diags := types.ListValueFrom(ctx, resource_report.FiltersValue{}.Type(ctx), values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom filters: %v", diags)
	}
	return list
}

func TestValidateReportAttributionQuery(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	configPath := path.Root("config")
	groupType := func(i int) path.Path { return configPath.AtName("group").AtListIndex(i).AtName("type") }
	groupID := func(i int) path.Path { return configPath.AtName("group").AtListIndex(i).AtName("id") }
	attribution := types.StringValue("attribution")

	nullGroup := types.ListNull(resource_report.GroupValue{}.Type(ctx))
	nullDimensions := types.ListNull(resource_report.DimensionsValue{}.Type(ctx))
	nullFilters := types.ListNull(resource_report.FiltersValue{}.Type(ctx))

	type lists struct{ group, dimensions, filters types.List }
	none := lists{nullGroup, nullDimensions, nullFilters}
	withGroup := func(dims ...attributionDim) lists {
		return lists{attributionGroupList(ctx, t, dims...), nullDimensions, nullFilters}
	}

	tests := []struct {
		name       string
		dataSource types.String
		lists      lists
		wantPaths  []path.Path
	}{
		// Valid configurations.
		{name: "no attribution at all", dataSource: types.StringNull(), lists: withGroup(plainDim("service_description", "fixed"))},
		{name: "attribution with tier in group", dataSource: attribution, lists: withGroup(tierDim("set_a"))},
		{name: "attribution with tier in dimensions", dataSource: attribution, lists: lists{nullGroup, attributionDimensionsList(ctx, t, tierDim("set_a")), nullFilters}},
		{name: "attribution with tier in filters", dataSource: attribution, lists: lists{nullGroup, nullDimensions, attributionFiltersList(ctx, t, tierDim("set_a"))}},
		{name: "attribution with tier and datetime dimensions", dataSource: attribution, lists: lists{attributionGroupList(ctx, t, tierDim("set_a")), attributionDimensionsList(ctx, t, plainDim("year", "datetime")), nullFilters}},
		{name: "same attribution across lists", dataSource: attribution, lists: lists{attributionGroupList(ctx, t, tierDim("set_a")), nullDimensions, attributionFiltersList(ctx, t, attributionDim{id: types.StringValue("set_a"), dimType: types.StringValue("attribution_tier_2")})}},
		{name: "deprecated attribution dimension type is not a tier", dataSource: types.StringValue("billing"), lists: withGroup(plainDim("rule_id", "attribution"))},

		// Deferral.
		{name: "unknown data source with tier", dataSource: types.StringUnknown(), lists: withGroup(tierDim("set_a"))},
		{name: "attribution with unknown group list", dataSource: attribution, lists: lists{types.ListUnknown(resource_report.GroupValue{}.Type(ctx)), nullDimensions, nullFilters}},
		{name: "attribution with unknown group element", dataSource: attribution, lists: withGroup(attributionDim{unknown: true})},
		{name: "attribution with unknown element type", dataSource: attribution, lists: withGroup(attributionDim{id: types.StringValue("set_a"), dimType: types.StringUnknown()})},
		{name: "unknown id does not mismatch", dataSource: attribution, lists: withGroup(tierDim("set_a"), attributionDim{id: types.StringUnknown(), dimType: types.StringValue("attribution_tier_2")})},

		// Invalid configurations.
		{name: "attribution without tier", dataSource: attribution, lists: none, wantPaths: []path.Path{configPath.AtName("data_source")}},
		{name: "attribution with only plain dimensions", dataSource: attribution, lists: withGroup(plainDim("service_description", "fixed")), wantPaths: []path.Path{configPath.AtName("data_source")}},
		{name: "tier with null data source", dataSource: types.StringNull(), lists: withGroup(tierDim("set_a")), wantPaths: []path.Path{groupType(0)}},
		{name: "tier with billing data source", dataSource: types.StringValue("billing"), lists: withGroup(plainDim("year", "datetime"), tierDim("set_a")), wantPaths: []path.Path{groupType(1)}},
		{name: "mismatched attribution ids", dataSource: attribution, lists: withGroup(tierDim("set_a"), tierDim("set_b"), tierDim("set_c")), wantPaths: []path.Path{groupID(1), groupID(2)}},
		{name: "mismatched ids reported despite unknown data source", dataSource: types.StringUnknown(), lists: withGroup(tierDim("set_a"), tierDim("set_b")), wantPaths: []path.Path{groupID(1)}},
		{name: "known-invalid tier after unknown element", dataSource: types.StringValue("billing"), lists: withGroup(attributionDim{unknown: true}, tierDim("set_a")), wantPaths: []path.Path{groupType(1)}},
		{name: "known-invalid tier before unknown element", dataSource: types.StringValue("billing"), lists: withGroup(tierDim("set_a"), attributionDim{unknown: true}), wantPaths: []path.Path{groupType(0)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			getter := fakeAttributeGetter{
				configPath.AtName("data_source").String(): test.dataSource,
				configPath.AtName("group").String():       test.lists.group,
				configPath.AtName("dimensions").String():  test.lists.dimensions,
				configPath.AtName("filters").String():     test.lists.filters,
			}
			var diags diag.Diagnostics
			validateReportAttributionQuery(t.Context(), getter, &diags)

			if got := len(diags); got != len(test.wantPaths) {
				t.Fatalf("diagnostics = %d, want %d: %v", got, len(test.wantPaths), diags)
			}
			for index, wantPath := range test.wantPaths {
				assertAttributeError(t, diags[index], invalidAttributionQuerySummary, wantPath)
			}
		})
	}
}

func TestReportResource_ModifyPlan_AttributionReportReadOnly(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	attributionConfig := func(currency string) map[string]attr.Value {
		cfg := lbcBaseConfigMap(ctx, t)
		cfg["data_source"] = types.StringValue("attribution")
		cfg["currency"] = types.StringValue(currency)
		return cfg
	}

	t.Run("update of an attribution report inherited from state is rejected", func(t *testing.T) {
		t.Parallel()

		// Config omits data_source, so the plan inherits "attribution" from state.
		state := attributionConfig("USD")
		plan := attributionConfig("EUR")
		cfg := attributionConfig("EUR")
		cfg["data_source"] = types.StringNull()

		resp := lbcModifyPlan(ctx, t, state, plan, cfg)
		if got := lbcCountErrors(resp.Diagnostics, attributionReportReadOnlySummary); got != 1 {
			t.Fatalf("expected one %q error, got %d: %v", attributionReportReadOnlySummary, got, resp.Diagnostics)
		}
	})

	t.Run("no-op plan of an attribution report is not rejected", func(t *testing.T) {
		t.Parallel()

		state := attributionConfig("USD")
		cfg := attributionConfig("USD")
		cfg["data_source"] = types.StringNull()

		resp := lbcModifyPlan(ctx, t, state, state, cfg)
		if got := lbcCountErrors(resp.Diagnostics, attributionReportReadOnlySummary); got != 0 {
			t.Fatalf("expected no %q error, got %d: %v", attributionReportReadOnlySummary, got, resp.Diagnostics)
		}
	})

	t.Run("update of a billing report is not rejected", func(t *testing.T) {
		t.Parallel()

		state := lbcBaseConfigMap(ctx, t)
		plan := lbcBaseConfigMap(ctx, t)
		plan["currency"] = types.StringValue("EUR")

		resp := lbcModifyPlan(ctx, t, state, plan, plan)
		if got := lbcCountErrors(resp.Diagnostics, attributionReportReadOnlySummary); got != 0 {
			t.Fatalf("expected no %q error, got %d: %v", attributionReportReadOnlySummary, got, resp.Diagnostics)
		}
	})
}
