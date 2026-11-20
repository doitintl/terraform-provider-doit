package provider

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_custom_metric"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TestCustomMetricVariableLetterFromPosition checks that the letter is planned
// from the element index, whatever the plan and state hold (a reorder makes the
// stored letter of an element wrong), and that a path without an index, or an
// index past E, leaves the plan alone.
func TestCustomMetricVariableLetterFromPosition(t *testing.T) {
	ctx := t.Context()
	letterAt := func(i int) path.Path { return path.Root("variables").AtListIndex(i).AtName("letter") }

	cases := []struct {
		name  string
		path  path.Path
		plan  types.String
		state types.String
		want  types.String
	}{
		{"first, create", letterAt(0), types.StringUnknown(), types.StringNull(), types.StringValue("A")},
		{"second, create", letterAt(1), types.StringUnknown(), types.StringNull(), types.StringValue("B")},
		{"third, unchanged", letterAt(2), types.StringValue("C"), types.StringValue("C"), types.StringValue("C")},
		{"fourth, reordered", letterAt(3), types.StringValue("A"), types.StringValue("A"), types.StringValue("D")},
		{"fifth, update", letterAt(4), types.StringUnknown(), types.StringValue("B"), types.StringValue("E")},
		{"sixth, rejected by the list validator", letterAt(5), types.StringUnknown(), types.StringNull(), types.StringUnknown()},
		{"no index", path.Root("letter"), types.StringUnknown(), types.StringNull(), types.StringUnknown()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				Path:        tc.path,
				ConfigValue: types.StringNull(),
				PlanValue:   tc.plan,
				StateValue:  tc.state,
			}
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
			customMetricVariableLetterFromPosition().PlanModifyString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tc.want) {
				t.Errorf("plan = %v, want %v", resp.PlanValue, tc.want)
			}
		})
	}
}

func TestCustomMetricDecimalsRangeValidator(t *testing.T) {
	ctx := t.Context()

	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"decimals": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"min": schema.Int64Attribute{Required: true},
				"max": schema.Int64Attribute{Required: true},
			},
		},
	}}
	decimalsType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"min": tftypes.Number, "max": tftypes.Number}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"decimals": decimalsType}}
	attrTypes := map[string]attr.Type{"min": types.Int64Type, "max": types.Int64Type}

	number := func(v any) tftypes.Value {
		switch v := v.(type) {
		case int:
			return tftypes.NewValue(tftypes.Number, v)
		case string: // "unknown"
			return tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)
		default:
			return tftypes.NewValue(tftypes.Number, nil)
		}
	}

	cases := []struct {
		name     string
		decimals tftypes.Value
		value    types.Object
		wantErr  bool
	}{
		{"null", tftypes.NewValue(decimalsType, nil), types.ObjectNull(attrTypes), false},
		{"unknown", tftypes.NewValue(decimalsType, tftypes.UnknownValue), types.ObjectUnknown(attrTypes), false},
		{"min unknown", tftypes.NewValue(decimalsType, map[string]tftypes.Value{"min": number("unknown"), "max": number(1)}),
			types.ObjectValueMust(attrTypes, map[string]attr.Value{"min": types.Int64Unknown(), "max": types.Int64Value(1)}), false},
		{"max unknown", tftypes.NewValue(decimalsType, map[string]tftypes.Value{"min": number(3), "max": number("unknown")}),
			types.ObjectValueMust(attrTypes, map[string]attr.Value{"min": types.Int64Value(3), "max": types.Int64Unknown()}), false},
		{"min below max", tftypes.NewValue(decimalsType, map[string]tftypes.Value{"min": number(0), "max": number(2)}),
			types.ObjectValueMust(attrTypes, map[string]attr.Value{"min": types.Int64Value(0), "max": types.Int64Value(2)}), false},
		{"min equals max", tftypes.NewValue(decimalsType, map[string]tftypes.Value{"min": number(2), "max": number(2)}),
			types.ObjectValueMust(attrTypes, map[string]attr.Value{"min": types.Int64Value(2), "max": types.Int64Value(2)}), false},
		{"min above max", tftypes.NewValue(decimalsType, map[string]tftypes.Value{"min": number(3), "max": number(1)}),
			types.ObjectValueMust(attrTypes, map[string]attr.Value{"min": types.Int64Value(3), "max": types.Int64Value(1)}), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.ObjectRequest{
				Path:        path.Root("decimals"),
				ConfigValue: tc.value,
				Config: tfsdk.Config{
					Schema: s,
					Raw:    tftypes.NewValue(rootType, map[string]tftypes.Value{"decimals": tc.decimals}),
				},
			}
			resp := &validator.ObjectResponse{}
			customMetricDecimalsRangeValidator{}.ValidateObject(ctx, req, resp)

			if !tc.wantErr {
				if len(resp.Diagnostics) != 0 {
					t.Fatalf("diagnostics = %v, want none", resp.Diagnostics)
				}
				return
			}
			if len(resp.Diagnostics) != 1 || resp.Diagnostics.ErrorsCount() != 1 {
				t.Fatalf("diagnostics = %v, want exactly one error", resp.Diagnostics)
			}
			d := resp.Diagnostics[0]
			if d.Summary() != "Invalid Decimals Range" {
				t.Errorf("summary = %q", d.Summary())
			}
			withPath, ok := d.(diag.DiagnosticWithPath)
			if !ok || !withPath.Path().Equal(path.Root("decimals")) {
				t.Errorf("diagnostic path = %v, want decimals", d)
			}
		})
	}
}

func newTestCustomMetricDecimals(ctx context.Context, minValue, maxValue int64) resource_custom_metric.DecimalsValue {
	return resource_custom_metric.NewDecimalsValueMust(
		resource_custom_metric.DecimalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"min": types.Int64Value(minValue), "max": types.Int64Value(maxValue)},
	)
}

func newTestCustomMetricVariables(ctx context.Context, t *testing.T, letters []attr.Value, metricTypes ...string) types.List {
	t.Helper()
	values := make([]attr.Value, 0, len(metricTypes))
	for i, metricType := range metricTypes {
		metric := resource_custom_metric.NewMetricValueMust(
			resource_custom_metric.MetricValue{}.AttributeTypes(ctx),
			map[string]attr.Value{"type": types.StringValue(metricType), "value": types.StringValue("cost")},
		)
		values = append(values, resource_custom_metric.NewVariablesValueMust(
			resource_custom_metric.VariablesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"allocation_id": types.StringValue("alloc-" + string(rune('a'+i))),
				"letter":        letters[i],
				"metric":        metric,
			},
		))
	}
	list, d := types.ListValue(resource_custom_metric.VariablesValue{}.Type(ctx), values)
	if d.HasError() {
		t.Fatalf("building variables: %v", d)
	}
	return list
}

// TestSetCustomMetricDecimalsUpdate checks the JSON of the decimals field in an
// update: a null plan clears it with an explicit null, a known one sends both
// bounds, and an unknown one is left out.
func TestSetCustomMetricDecimalsUpdate(t *testing.T) {
	ctx := t.Context()

	cases := []struct {
		name     string
		decimals resource_custom_metric.DecimalsValue
		want     string
	}{
		{"null clears", resource_custom_metric.NewDecimalsValueNull(), `{"decimals":null}`},
		{"known sets both bounds", newTestCustomMetricDecimals(ctx, 1, 3), `{"decimals":{"max":3,"min":1}}`},
		{"unknown is left out", resource_custom_metric.NewDecimalsValueUnknown(), `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req models.UpdateCustomMetricApplicationMergePatchPlusJSONRequestBody
			setCustomMetricDecimalsUpdate(&req, tc.decimals)

			got, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("JSON = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestCustomMetricRequests checks that create and update send every
// configured field, never the letters, and that create leaves out a null
// decimals.
func TestCustomMetricRequests(t *testing.T) {
	ctx := t.Context()

	plan := customMetricResourceModel{
		Name:        types.StringValue("Cost per ride"),
		Description: types.StringValue(""),
		Formula:     types.StringValue("A / B"),
		UnitType:    types.StringValue("percentage"),
		Decimals:    resource_custom_metric.NewDecimalsValueNull(),
		Variables: newTestCustomMetricVariables(ctx, t,
			[]attr.Value{types.StringValue("A"), types.StringUnknown()}, "basic", "extended"),
	}

	const wantVariables = `[{"allocationId":"alloc-a","metric":{"type":"basic","value":"cost"}},{"allocationId":"alloc-b","metric":{"type":"extended","value":"cost"}}]`

	createReq, d := plan.toCreateRequest(ctx)
	if d.HasError() {
		t.Fatalf("toCreateRequest: %v", d)
	}
	got, err := json.Marshal(createReq)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"description":"","formula":"A / B","name":"Cost per ride","unitType":"percentage","variables":` + wantVariables + `}`; string(got) != want {
		t.Errorf("create JSON =\n%s\nwant\n%s", got, want)
	}

	updateReq, d := plan.toUpdateRequest(ctx)
	if d.HasError() {
		t.Fatalf("toUpdateRequest: %v", d)
	}
	got, err = json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"decimals":null,"description":"","formula":"A / B","name":"Cost per ride","unitType":"percentage","variables":` + wantVariables + `}`; string(got) != want {
		t.Errorf("update JSON =\n%s\nwant\n%s", got, want)
	}
}

func testCustomMetricResponse(decimals nullable.Nullable[models.CustomMetricDecimals]) *models.CustomMetric {
	created := time.Date(2026, 11, 19, 10, 27, 3, 77_000_000, time.UTC)
	updated := time.Date(2026, 11, 19, 10, 37, 46, 765_000_000, time.UTC)
	owner := openapi_types.Email("owner@example.com")
	return &models.CustomMetric{
		Id:          new("m1"),
		Etag:        new(`"etag-2"`),
		Name:        "Cost per ride",
		Description: "",
		Owner:       &owner,
		Formula:     "A + B",
		UnitType:    models.CustomMetricUnitTypeNumber,
		Decimals:    decimals,
		CreateTime:  &created,
		UpdateTime:  &updated,
		Variables: []models.CustomMetricVariable{
			{AllocationId: "alloc-a", Letter: new("A"), Metric: models.CustomMetricVariableMetric{Type: models.CustomMetricVariableMetricTypeBasic, Value: "cost"}},
			{AllocationId: "alloc-b", Letter: new("B"), Metric: models.CustomMetricVariableMetric{Type: models.CustomMetricVariableMetricTypeExtended, Value: "cost"}},
		},
	}
}

func TestMapCustomMetricToModel(t *testing.T) {
	ctx := t.Context()

	cases := []struct {
		name     string
		decimals nullable.Nullable[models.CustomMetricDecimals]
		want     resource_custom_metric.DecimalsValue
	}{
		{"decimals null", nullable.NewNullNullable[models.CustomMetricDecimals](), resource_custom_metric.NewDecimalsValueNull()},
		{"decimals set", valueToNullable(models.CustomMetricDecimals{Min: 0, Max: 2}), newTestCustomMetricDecimals(ctx, 0, 2)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var state customMetricResourceModel
			if d := mapCustomMetricToModel(ctx, testCustomMetricResponse(tc.decimals), &state); d.HasError() {
				t.Fatalf("mapCustomMetricToModel: %v", d)
			}

			if !state.Decimals.Equal(tc.want) {
				t.Errorf("decimals = %v, want %v", state.Decimals, tc.want)
			}
			wantVariables := newTestCustomMetricVariables(ctx, t,
				[]attr.Value{types.StringValue("A"), types.StringValue("B")}, "basic", "extended")
			if !state.Variables.Equal(wantVariables) {
				t.Errorf("variables = %v, want %v", state.Variables, wantVariables)
			}
			for name, got := range map[string]types.String{
				"id":          state.Id,
				"etag":        state.Etag,
				"owner":       state.Owner,
				"formula":     state.Formula,
				"unit_type":   state.UnitType,
				"description": state.Description,
				"create_time": state.CreateTime,
				"update_time": state.UpdateTime,
			} {
				want := map[string]string{
					"id":          "m1",
					"etag":        `"etag-2"`,
					"owner":       "owner@example.com",
					"formula":     "A + B",
					"unit_type":   "number",
					"description": "",
					"create_time": "2026-11-19T10:27:03.077Z",
					"update_time": "2026-11-19T10:37:46.765Z",
				}[name]
				if !got.Equal(types.StringValue(want)) {
					t.Errorf("%s = %v, want %q", name, got, want)
				}
			}
		})
	}
}

// TestOverlayCustomMetricComputedFields checks the overlay on an update: the
// planned letters and etag are kept when known, unknown ones are resolved,
// and the planned decimals is never replaced by the response.
func TestOverlayCustomMetricComputedFields(t *testing.T) {
	ctx := t.Context()

	plan := customMetricResourceModel{
		Name:        types.StringValue("Cost per ride"),
		Description: types.StringValue(""),
		Formula:     types.StringValue("A + B"),
		UnitType:    types.StringValue("number"),
		Decimals:    resource_custom_metric.NewDecimalsValueNull(),
		Id:          types.StringValue("m1"),
		Owner:       types.StringValue("owner@example.com"),
		CreateTime:  types.StringValue("2026-11-19T10:27:03.077Z"),
		Etag:        types.StringUnknown(),
		UpdateTime:  types.StringUnknown(),
		Variables: newTestCustomMetricVariables(ctx, t,
			[]attr.Value{types.StringValue("A"), types.StringUnknown()}, "basic", "extended"),
	}

	// The response holds decimals the plan does not: the plan value wins.
	resp := testCustomMetricResponse(valueToNullable(models.CustomMetricDecimals{Min: 1, Max: 1}))
	if d := overlayCustomMetricComputedFields(ctx, resp, &plan); d.HasError() {
		t.Fatalf("overlay: %v", d)
	}

	if !plan.Decimals.Equal(resource_custom_metric.NewDecimalsValueNull()) {
		t.Errorf("decimals = %v, want null", plan.Decimals)
	}
	if !plan.Etag.Equal(types.StringValue(`"etag-2"`)) {
		t.Errorf("etag = %v", plan.Etag)
	}
	if !plan.UpdateTime.Equal(types.StringValue("2026-11-19T10:37:46.765Z")) {
		t.Errorf("update_time = %v", plan.UpdateTime)
	}
	wantVariables := newTestCustomMetricVariables(ctx, t,
		[]attr.Value{types.StringValue("A"), types.StringValue("B")}, "basic", "extended")
	if !plan.Variables.Equal(wantVariables) {
		t.Errorf("variables = %v, want %v", plan.Variables, wantVariables)
	}
}
