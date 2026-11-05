package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// modelsEnumValues returns the string values of the constants declared with the
// given type in models/models_gen.go, so a hand-maintained list can be compared
// with the spec-generated enum it mirrors. A value the API adds then fails here
// instead of being rejected, or silently unrecognised, by the provider.
func modelsEnumValues(t *testing.T, typeName string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "models/models_gen.go", nil, 0)
	if err != nil {
		t.Fatalf("parse models_gen.go: %v", err)
	}
	var values []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			ident, ok := valueSpec.Type.(*ast.Ident)
			if !ok || ident.Name != typeName {
				continue
			}
			for _, expr := range valueSpec.Values {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", lit.Value, err)
				}
				values = append(values, value)
			}
		}
	}
	if len(values) == 0 {
		t.Fatalf("no constants of type %s found in models_gen.go", typeName)
	}
	slices.Sort(values)
	return values
}

func assertSameValues(t *testing.T, name string, got, want []string) {
	t.Helper()
	got = slices.Sorted(slices.Values(got))
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want the spec enum %v", name, got, want)
	}
}

func TestReportQueryListsMatchModelsEnums(t *testing.T) {
	assertSameValues(t, "reportQueryDataSources", reportQueryDataSources,
		modelsEnumValues(t, "CloudAnalyticsExternalConfigDataSource"))
	assertSameValues(t, "reportQueryDimensionsTypes", reportQueryDimensionsTypes,
		modelsEnumValues(t, "DimensionsTypes"))
}

func TestAttributionTierTypesMatchModelsEnum(t *testing.T) {
	var tiers []string
	for _, value := range modelsEnumValues(t, "DimensionsTypes") {
		if strings.HasPrefix(value, "attribution_tier_") {
			tiers = append(tiers, value)
		}
		if got, want := isAttributionTierType(value), strings.HasPrefix(value, "attribution_tier_"); got != want {
			t.Errorf("isAttributionTierType(%q) = %v, want %v", value, got, want)
		}
	}
	if len(tiers) == 0 {
		t.Fatal("no attribution tier values in the DimensionsTypes enum")
	}
}

func TestCustomerSubscriptionAttachmentValuesMatchModelsEnum(t *testing.T) {
	assertSameValues(t, "customerSubscriptionAttachmentValues", customerSubscriptionAttachmentValues,
		modelsEnumValues(t, "CustomerSubscriptionAttachment"))
}

// The label assignment object_type list is written out in the schema, so every
// value of the spec enum has to pass its validator.
func TestLabelAssignmentObjectTypesAcceptModelsEnum(t *testing.T) {
	ctx := t.Context()
	var resp resource.SchemaResponse
	(&labelAssignmentsResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	assignments, ok := resp.Schema.Attributes["assignments"].(schema.SetNestedAttribute)
	if !ok {
		t.Fatalf("assignments is %T, want SetNestedAttribute", resp.Schema.Attributes["assignments"])
	}
	objectType, ok := assignments.NestedObject.Attributes["object_type"].(schema.StringAttribute)
	if !ok {
		t.Fatal("object_type must be a StringAttribute")
	}

	accepts := func(value string) bool {
		for _, v := range objectType.Validators {
			var vr validator.StringResponse
			v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(value)}, &vr)
			if vr.Diagnostics.HasError() {
				return false
			}
		}
		return true
	}
	if accepts("not-an-object-type") {
		t.Fatal("object_type accepts an invalid value; the validator is not attached")
	}
	for _, value := range modelsEnumValues(t, "LabelAssignmentObjectObjectType") {
		if !accepts(value) {
			t.Errorf("object_type rejects %q, which the API spec allows", value)
		}
	}
}
