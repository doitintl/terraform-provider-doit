package provider

import (
	"context"
	"reflect"
	"regexp"
	"slices"
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

// validateStringValue runs every validator of a string attribute against value
// and returns the diagnostics they produce.
func validateStringValue(ctx context.Context, validators []validator.String, attributePath path.Path, value string) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, candidate := range validators {
		response := &validator.StringResponse{}
		candidate.ValidateString(ctx, validator.StringRequest{Path: attributePath, ConfigValue: types.StringValue(value)}, response)
		diags.Append(response.Diagnostics...)
	}
	return diags
}

var attributionTierTypes = []string{"attribution_tier_1", "attribution_tier_2", "attribution_tier_3"}

// The generated enums of doit_report, doit_alert and doit_budget leave out the
// Attribute values, which only an ad-hoc query can use, and the schema notes
// point users to doit_report_query.
func TestAttributionValuesRejectedByGeneratedSchemas(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tests := []struct {
		name      string
		resource  resourceWithSchema
		typePaths []string
		dataPath  string
	}{
		{name: "report", resource: &reportResource{}, typePaths: reportDimensionsTypeAttributePaths, dataPath: "config.data_source"},
		{name: "alert", resource: &alertResource{}, typePaths: []string{"config.scopes[*].type"}},
		{name: "budget", resource: &budgetResource{}, typePaths: []string{"scopes[*].type"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			s := resourceSchemaForTest(ctx, test.resource)
			for _, attributePath := range test.typePaths {
				attribute := resourceStringAttributeAtPath(t, s.Attributes, attributePath)
				for _, value := range attributionTierTypes {
					if !validateStringValue(ctx, attribute.Validators, path.Root("type"), value).HasError() {
						t.Errorf("%s: %q is accepted, want an error", attributePath, value)
					}
				}
				// The deprecated attribution types are a different thing and stay valid.
				for _, value := range []string{"fixed", "allocation_rule", "attribution", "attribution_group"} {
					if diags := validateStringValue(ctx, attribute.Validators, path.Root("type"), value); diags.HasError() {
						t.Errorf("%s: %q is rejected: %v", attributePath, value, diags)
					}
				}
				if !strings.Contains(attribute.MarkdownDescription, attributionTierTypeNote) {
					t.Errorf("%s: markdown description lacks the tier note", attributePath)
				}
				if !strings.Contains(attribute.Description, attributionTierTypePlainNote) {
					t.Errorf("%s: description lacks the tier note", attributePath)
				}
			}

			if test.dataPath == "" {
				return
			}
			attribute := resourceStringAttributeAtPath(t, s.Attributes, test.dataPath)
			if !validateStringValue(ctx, attribute.Validators, path.Root("data_source"), attributionReportDataSource).HasError() {
				t.Errorf("%s: %q is accepted, want an error", test.dataPath, attributionReportDataSource)
			}
			for _, value := range []string{"billing", "billing-datahub"} {
				if diags := validateStringValue(ctx, attribute.Validators, path.Root("data_source"), value); diags.HasError() {
					t.Errorf("%s: %q is rejected: %v", test.dataPath, value, diags)
				}
			}
			if !strings.Contains(attribute.MarkdownDescription, attributionDataSourceReadOnlyNote) {
				t.Errorf("%s: markdown description lacks the read-only note", test.dataPath)
			}
		})
	}
}

// doit_report_query is the one place attribution queries work. It derives its
// schema from the report resource, so it has to restore the values that schema
// leaves out.
func TestReportQueryAcceptsAttributionValues(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	var response datasource.SchemaResponse
	(&reportQueryDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("building report query schema: %v", response.Diagnostics.Errors())
	}

	for _, attributePath := range reportDimensionsTypeAttributePaths {
		attribute := dataSourceStringAttributeAtPath(t, response.Schema.Attributes, attributePath)
		for _, value := range reportQueryDimensionsTypes {
			if diags := validateStringValue(ctx, attribute.Validators, path.Root("type"), value); diags.HasError() {
				t.Errorf("%s: %q is rejected: %v", attributePath, value, diags)
			}
		}
		if !validateStringValue(ctx, attribute.Validators, path.Root("type"), "not_a_type").HasError() {
			t.Errorf("%s: an unknown type is accepted", attributePath)
		}
		assertHasDeprecatedDimensionsTypeValidator(t, attribute.Validators)
		if strings.Contains(attribute.MarkdownDescription, attributionTierTypeNote) {
			t.Errorf("%s: carries the resource-only tier note", attributePath)
		}
		if !strings.Contains(attribute.MarkdownDescription, attributionQueryTierTypeNote) {
			t.Errorf("%s: markdown description lacks the query tier note", attributePath)
		}
		for _, value := range attributionTierTypes {
			if !strings.Contains(attribute.Description, "`"+value+"`") {
				t.Errorf("%s: description does not list %q", attributePath, value)
			}
		}
	}

	attribute := dataSourceStringAttributeAtPath(t, response.Schema.Attributes, "config.data_source")
	for _, value := range reportQueryDataSources {
		if diags := validateStringValue(ctx, attribute.Validators, path.Root("data_source"), value); diags.HasError() {
			t.Errorf("config.data_source: %q is rejected: %v", value, diags)
		}
	}
	if !validateStringValue(ctx, attribute.Validators, path.Root("data_source"), "not_a_source").HasError() {
		t.Error("config.data_source: an unknown data source is accepted")
	}
	if strings.Contains(attribute.MarkdownDescription, attributionDataSourceReadOnlyNote) {
		t.Error("config.data_source: carries the resource-only read-only note")
	}
	if !strings.Contains(attribute.MarkdownDescription, attributionQueryDataSourceNote) {
		t.Error("config.data_source: markdown description lacks the query note")
	}

	found := false
	for _, configValidator := range (&reportQueryDataSource{}).ConfigValidators(ctx) {
		if _, ok := configValidator.(reportAttributionQueryDataSourceValidator); ok {
			found = true
		}
	}
	if !found {
		t.Error("report query does not register reportAttributionQueryDataSourceValidator")
	}
}

var oneOfValuePattern = regexp.MustCompile(`"([^"]+)"`)

// oneOfValues returns the values of the generated stringvalidator.OneOf among
// validators, read from its description.
func oneOfValues(ctx context.Context, t *testing.T, validators []validator.String) []string {
	t.Helper()

	for _, candidate := range validators {
		description := candidate.Description(ctx)
		if !strings.HasPrefix(description, "value must be one of:") {
			continue
		}

		var values []string
		for _, match := range oneOfValuePattern.FindAllStringSubmatch(description, -1) {
			values = append(values, match[1])
		}
		return values
	}

	t.Fatal("no OneOf validator among the attribute's validators")
	return nil
}

// The query schema lists its enums explicitly, so a value that upstream adds to
// the report resource's enums must also be added there.
func TestReportQueryAttributionValuesExtendReportResource(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	resourceAttributes := resourceSchemaForTest(ctx, &reportResource{}).Attributes

	tests := []struct {
		name   string
		paths  []string
		want   []string
		extras []string
	}{
		{name: "data source", paths: []string{"config.data_source"}, want: reportQueryDataSources, extras: []string{attributionReportDataSource}},
		{name: "dimension types", paths: reportDimensionsTypeAttributePaths, want: reportQueryDimensionsTypes, extras: attributionTierTypes},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			wantFromResource := slices.DeleteFunc(slices.Clone(test.want), func(value string) bool { return slices.Contains(test.extras, value) })
			for _, attributePath := range test.paths {
				got := oneOfValues(ctx, t, resourceStringAttributeAtPath(t, resourceAttributes, attributePath).Validators)
				if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(wantFromResource))) {
					t.Errorf("%s: report resource accepts %v, the query schema extends %v", attributePath, got, wantFromResource)
				}
			}
			for _, extra := range test.extras {
				if !slices.Contains(test.want, extra) {
					t.Errorf("query values lack %q", extra)
				}
			}
		})
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
	for _, value := range attributionTierTypes {
		if diags := validateStringValue(t.Context(), attribute.Validators, path.Root("type"), value); diags.HasError() {
			t.Errorf("%q is rejected: %v", value, diags)
		}
	}
	if !strings.Contains(attribute.MarkdownDescription, attributionTierDimensionNote) {
		t.Error("markdown description lacks the tier note")
	}
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
