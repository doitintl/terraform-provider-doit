package provider

import (
	"fmt"
	"strings"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
// dimensions without it, and alert and budget scopes do not take tier
// dimensions. The API spec gives those requests their own enums without the
// Attribute values, so the generated schemas of doit_report, doit_alert and
// doit_budget already reject them. doit_report_query derives its schema from
// doit_report, so it restores the values here.

const (
	attributionReportDataSource = string(models.CloudAnalyticsExternalConfigDataSourceAttribution)

	attributionReportReadOnlySummary = "Attribution Reports Are Read-Only"

	attributeAttributionsGuideURL = "https://registry.terraform.io/providers/doitintl/doit/latest/docs/guides/attribute_attributions"
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

var attributionReportReadOnlyDetail = fmt.Sprintf(
	"The %q data source queries DoiT Attribute attributions, and the DoiT API cannot create or update "+
		"reports that use it. Run the query with the doit_report_query data source instead; saved "+
		"attribution reports can be read with the doit_report data source. See %s",
	attributionReportDataSource,
	attributeAttributionsGuideURL,
)

// Values doit_report_query accepts. The report resource's generated schema leaves
// out the Attribute values, which only an ad-hoc query can use, so the query
// schema lists its own. TestReportQueryAttributionValuesExtendReportResource
// keeps these in step with the generated enums.
var (
	reportQueryDataSources = []string{
		string(models.CloudAnalyticsExternalConfigDataSourceBilling),
		string(models.CloudAnalyticsExternalConfigDataSourceBqlens),
		string(models.CloudAnalyticsExternalConfigDataSourceBillingDatahub),
		string(models.CloudAnalyticsExternalConfigDataSourceKubernetesUtilization),
		string(models.CloudAnalyticsExternalConfigDataSourceAttribution),
	}

	reportQueryDimensionsTypes = []string{
		string(models.DimensionsTypesDatetime),
		string(models.DimensionsTypesFixed),
		string(models.DimensionsTypesOptional),
		string(models.DimensionsTypesLabel),
		string(models.DimensionsTypesTag),
		string(models.DimensionsTypesProjectLabel),
		string(models.DimensionsTypesSystemLabel),
		string(models.DimensionsTypesAttribution),
		string(models.DimensionsTypesAttributionGroup),
		string(models.DimensionsTypesAllocation),
		string(models.DimensionsTypesAllocationRule),
		string(models.DimensionsTypesGke),
		string(models.DimensionsTypesGkeLabel),
		string(models.DimensionsTypesAttributionTier1),
		string(models.DimensionsTypesAttributionTier2),
		string(models.DimensionsTypesAttributionTier3),
	}
)

// Documentation notes appended to the generated attribute descriptions of the
// resources that cannot use the Attribute values.
const (
	attributionTierTypeNote = "The DoiT Attribute attribution tier types (`attribution_tier_1`, `attribution_tier_2` " +
		"and `attribution_tier_3`) are only available in the `doit_report_query` data source. They are unrelated " +
		"to the deprecated `attribution` type. See the [Attribute attributions guide](../guides/attribute_attributions)."
	attributionTierTypePlainNote = "The DoiT Attribute attribution tier types (attribution_tier_1, attribution_tier_2 " +
		"and attribution_tier_3) are only available in the doit_report_query data source. They are unrelated " +
		"to the deprecated attribution type."

	attributionDataSourceReadOnlyNote = "The DoiT Attribute `attribution` data source is only available in the " +
		"`doit_report_query` data source, because the API cannot save attribution reports. " +
		"See the [Attribute attributions guide](../guides/attribute_attributions)."
	attributionDataSourceReadOnlyPlainNote = "The DoiT Attribute attribution data source is only available in the " +
		"doit_report_query data source, because the API cannot save attribution reports."
)

// describeAttributionTierTypes appends the tier type note to the dimension type
// attributes at attributePaths, whose generated enum leaves the tier types out.
func describeAttributionTierTypes(attributes map[string]schema.Attribute, attributePaths ...string) {
	updateStringAttributesAtPaths(attributes, "describeAttributionTierTypes", func(attribute schema.StringAttribute) schema.StringAttribute {
		attribute.Description = appendDescriptionNote(attribute.Description, attributionTierTypePlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(attribute.MarkdownDescription, attributionTierTypeNote)
		return attribute
	}, attributePaths...)
}

// describeAttributionReportDataSource appends the read-only note to the report
// data source attribute, whose generated enum leaves "attribution" out.
func describeAttributionReportDataSource(attributes map[string]schema.Attribute, attributePath string) {
	updateStringAttributesAtPaths(attributes, "describeAttributionReportDataSource", func(attribute schema.StringAttribute) schema.StringAttribute {
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

// Documentation for doit_report_query, the one place attribution queries work.
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

// possibleValuesLine formats an enum the way the generated descriptions do.
func possibleValuesLine(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "`" + value + "`"
	}
	return "Possible values: " + strings.Join(quoted, ", ")
}

// allowAttributionQueryValues makes the report config attributes that
// doit_report_query derives its schema from accept the Attribute values and
// describes them. It replaces the generated enum validators and descriptions of
// config.data_source and the dimension type attributes, because those describe
// report create and update requests, which reject the Attribute values.
func allowAttributionQueryValues(attributes map[string]schema.Attribute) {
	updateStringAttributesAtPaths(attributes, "allowAttributionQueryValues", func(attribute schema.StringAttribute) schema.StringAttribute {
		description := "Data source of the query.\n" +
			"When omitted, the API resolves the data source to `billing-datahub` if the customer has DataHub metrics configured, otherwise `billing`.\n" +
			possibleValuesLine(reportQueryDataSources)
		attribute.Validators = []validator.String{stringvalidator.OneOf(reportQueryDataSources...)}
		attribute.Description = appendDescriptionNote(description, attributionQueryDataSourcePlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(description, attributionQueryDataSourceNote)
		return attribute
	}, "config.data_source")

	updateStringAttributesAtPaths(attributes, "allowAttributionQueryValues", func(attribute schema.StringAttribute) schema.StringAttribute {
		description := "Dimension type. Pair it with `id`; list the valid `id` and `type` pairs of your account with the " +
			"`doit_dimensions` data source. `allocation_rule` replaces `attribution`; `allocation` replaces `attribution_group`.\n" +
			possibleValuesLine(reportQueryDimensionsTypes)
		attribute.Validators = []validator.String{stringvalidator.OneOf(reportQueryDimensionsTypes...)}
		attribute.Description = appendDescriptionNote(description, attributionQueryTierTypePlainNote)
		attribute.MarkdownDescription = appendDescriptionNote(description, attributionQueryTierTypeNote)
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
