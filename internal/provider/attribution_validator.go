package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// DoiT Attribute attributions (Attribution Center sets) are queried with the
// report data source "attribution" and the dimension types
// attribution_tier_1/2/3. They are unrelated to the deprecated "attribution"
// and "attribution_group" dimension types, which are the old names of
// allocation rules and allocations (see dimensions_type_validator.go).
//
// The API only runs them as ad-hoc queries (doit_report_query): report
// create/PATCH reject the "attribution" data source, reports reject tier
// dimensions without it, alert scopes reject tier dimensions, and budgets
// accept tier scopes although budgets reject the "attribution" data source
// those dimensions need.

const (
	attributionReportDataSource = string(models.CloudAnalyticsExternalConfigDataSourceAttribution)

	unsupportedAttributionTierTypeSummary = "Unsupported Dimension Type"
	attributionReportReadOnlySummary      = "Attribution Reports Are Read-Only"

	attributeAttributionsGuideURL = "https://registry.terraform.io/providers/doitintl/doit/latest/docs/guides/attribute_attributions"
)

var (
	_ validator.String = unsupportedAttributionTierTypeValidator{}
	_ validator.String = attributionReportDataSourceValidator{}
)

// isAttributionTierType reports whether value is one of the Attribute
// attribution tier dimension types.
func isAttributionTierType(value string) bool {
	switch models.DimensionsTypes(value) {
	case models.DimensionsTypesAttributionTier1, models.DimensionsTypesAttributionTier2, models.DimensionsTypesAttributionTier3:
		return true
	default:
		return false
	}
}

// unsupportedAttributionTierTypeValidator rejects the Attribute attribution
// tier dimension types on resources where the API cannot use them.
type unsupportedAttributionTierTypeValidator struct {
	// typeName is the Terraform type the attribute belongs to, e.g. "doit_alert".
	typeName string
}

func (v unsupportedAttributionTierTypeValidator) Description(_ context.Context) string {
	return fmt.Sprintf("Rejects the Attribute attribution tier dimension types, which %s does not support.", v.typeName)
}

func (v unsupportedAttributionTierTypeValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Rejects the Attribute attribution tier dimension types, which `%s` does not support.", v.typeName)
}

func (v unsupportedAttributionTierTypeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if !isAttributionTierType(value) {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		unsupportedAttributionTierTypeSummary,
		fmt.Sprintf(
			"Dimension type %q selects a DoiT Attribute attribution, which %s does not support. "+
				"Attribution tier dimensions only work in the doit_report_query data source with "+
				"data_source = %q. They are unrelated to the deprecated dimension type \"attribution\", "+
				"which is the old name of \"allocation_rule\". See %s",
			value,
			v.typeName,
			attributionReportDataSource,
			attributeAttributionsGuideURL,
		),
	)
}

// attributionReportDataSourceValidator rejects data_source = "attribution" on
// the doit_report resource: the API returns 400 for report create and update
// with that data source, so attribution reports can only be run ad hoc.
type attributionReportDataSourceValidator struct{}

func (v attributionReportDataSourceValidator) Description(_ context.Context) string {
	return "Rejects the read-only \"attribution\" data source."
}

func (v attributionReportDataSourceValidator) MarkdownDescription(_ context.Context) string {
	return "Rejects the read-only `attribution` data source."
}

func (v attributionReportDataSourceValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if req.ConfigValue.ValueString() != attributionReportDataSource {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		attributionReportReadOnlySummary,
		attributionReportReadOnlyDetail,
	)
}

var attributionReportReadOnlyDetail = fmt.Sprintf(
	"The %q data source queries DoiT Attribute attributions, and the DoiT API cannot create or update "+
		"reports that use it. Run the query with the doit_report_query data source instead; saved "+
		"attribution reports can be read with the doit_report data source. See %s",
	attributionReportDataSource,
	attributeAttributionsGuideURL,
)

// Documentation notes appended to the generated attribute descriptions, so the
// rendered docs explain why values listed under "Possible values" are rejected.
const (
	attributionTierUnsupportedNote = "The `attribution_tier_1`, `attribution_tier_2` and `attribution_tier_3` " +
		"types select DoiT Attribute attributions and are rejected here: they only work in the " +
		"`doit_report_query` data source with `data_source = \"attribution\"`. They are unrelated to the " +
		"deprecated `attribution` type. See the [Attribute attributions guide](../guides/attribute_attributions)."
	attributionTierUnsupportedPlainNote = "The attribution_tier_1, attribution_tier_2 and attribution_tier_3 " +
		"types select DoiT Attribute attributions and are rejected here: they only work in the " +
		"doit_report_query data source with data_source = \"attribution\". They are unrelated to the " +
		"deprecated attribution type."

	attributionDataSourceReadOnlyNote = "`attribution` (DoiT Attribute attributions) is rejected here because " +
		"the API cannot save attribution reports; query them with the `doit_report_query` data source. " +
		"See the [Attribute attributions guide](../guides/attribute_attributions)."
	attributionDataSourceReadOnlyPlainNote = "attribution (DoiT Attribute attributions) is rejected here because " +
		"the API cannot save attribution reports; query them with the doit_report_query data source."
)

// rejectAttributionTierTypes adds unsupportedAttributionTierTypeValidator and
// its documentation note to the dimension type attributes at attributePaths.
func rejectAttributionTierTypes(attributes map[string]schema.Attribute, typeName string, attributePaths ...string) {
	updateStringAttributesAtPaths(attributes, "rejectAttributionTierTypes", func(attribute schema.StringAttribute) schema.StringAttribute {
		attribute.Validators = append(attribute.Validators, unsupportedAttributionTierTypeValidator{typeName: typeName})
		attribute.Description = appendDescriptionNote(attribute.Description, attributionTierUnsupportedPlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(attribute.MarkdownDescription, attributionTierUnsupportedNote)
		return attribute
	}, attributePaths...)
}

// rejectAttributionReportDataSource adds attributionReportDataSourceValidator
// and its documentation note to the report data source attribute.
func rejectAttributionReportDataSource(attributes map[string]schema.Attribute, attributePath string) {
	updateStringAttributesAtPaths(attributes, "rejectAttributionReportDataSource", func(attribute schema.StringAttribute) schema.StringAttribute {
		attribute.Validators = append(attribute.Validators, attributionReportDataSourceValidator{})
		attribute.Description = appendDescriptionNote(attribute.Description, attributionDataSourceReadOnlyPlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(attribute.MarkdownDescription, attributionDataSourceReadOnlyNote)
		return attribute
	}, attributePath)
}

// appendDescriptionNote appends note as its own line of a generated description.
func appendDescriptionNote(description, note string) string {
	if description == "" {
		return note
	}
	return description + "\n" + note
}

// Documentation notes for doit_report_query, the one place attribution queries work.
const (
	attributionQueryDataSourceNote = "`attribution` queries one DoiT Attribute attribution. It needs an Attribute " +
		"subscription and at least one `attribution_tier_1`, `attribution_tier_2` or `attribution_tier_3` dimension " +
		"in `group`, `dimensions` or `filters`, all with the same attribution `id`. See the " +
		"[Attribute attributions guide](../guides/attribute_attributions)."
	attributionQueryDataSourcePlainNote = "attribution queries one DoiT Attribute attribution. It needs an Attribute " +
		"subscription and at least one attribution_tier_1, attribution_tier_2 or attribution_tier_3 dimension " +
		"in group, dimensions or filters, all with the same attribution id."

	attributionQueryTierTypeNote = "The `attribution_tier_1`, `attribution_tier_2` and `attribution_tier_3` types " +
		"select DoiT Attribute attributions and require `data_source = \"attribution\"`; their `id` is the " +
		"attribution set ID with `-` replaced by `_`. They are unrelated to the deprecated `attribution` type. " +
		"See the [Attribute attributions guide](../guides/attribute_attributions)."
	attributionQueryTierTypePlainNote = "The attribution_tier_1, attribution_tier_2 and attribution_tier_3 types " +
		"select DoiT Attribute attributions and require data_source = \"attribution\"; their id is the " +
		"attribution set ID with - replaced by _. They are unrelated to the deprecated attribution type."
)

// describeAttributionQuerySchema adds the attribution query notes to the report
// config attributes that doit_report_query derives its schema from.
func describeAttributionQuerySchema(attributes map[string]schema.Attribute) {
	updateStringAttributesAtPaths(attributes, "describeAttributionQuerySchema", func(attribute schema.StringAttribute) schema.StringAttribute {
		attribute.Description = appendDescriptionNote(attribute.Description, attributionQueryDataSourcePlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(attribute.MarkdownDescription, attributionQueryDataSourceNote)
		return attribute
	}, "config.data_source")
	updateStringAttributesAtPaths(attributes, "describeAttributionQuerySchema", func(attribute schema.StringAttribute) schema.StringAttribute {
		attribute.Description = appendDescriptionNote(attribute.Description, attributionQueryTierTypePlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(attribute.MarkdownDescription, attributionQueryTierTypeNote)
		return attribute
	}, reportDimensionsTypeAttributePaths...)
}

// Documentation note for the doit_dimension type input.
const (
	attributionTierDimensionNote = "The `attribution_tier_1`, `attribution_tier_2` and `attribution_tier_3` types " +
		"select DoiT Attribute attributions; use the `doit_dimensions` data source to list the ones your account " +
		"has data for. They are unrelated to the deprecated `attribution` type. See the " +
		"[Attribute attributions guide](../guides/attribute_attributions)."
	attributionTierDimensionPlainNote = "The attribution_tier_1, attribution_tier_2 and attribution_tier_3 types " +
		"select DoiT Attribute attributions; use the doit_dimensions data source to list the ones your account " +
		"has data for. They are unrelated to the deprecated attribution type."
)
