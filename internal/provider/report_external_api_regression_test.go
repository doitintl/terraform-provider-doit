// Regression tests for two bugs in the external Cloud Analytics API
// (`/analytics/v1/...`), the surface this provider talks to.
//
// Tracked upstream as CMP-49333 (follow-up to CMP-47539), fixed by
// doiteng/omni#60621. Until that fix is deployed, every test in this file
// except the *WithUnit control is expected to FAIL against the live API:
//
//  1. `timeRange.mode = "custom"` without a `unit` is rejected, because
//     TimeSettings.Validate() requires a unit for every mode. The provider
//     omits `unit` from the payload when it is not configured, and the
//     OpenAPI spec marks none of the timeRange fields required — so this is
//     a spec-legal request the API refuses. Reaches both the report resource
//     (create/update) and the report_query data source, since both flow
//     through MergeConfigWithExternalConfig.
//
//  2. Queries requesting several metrics return only the first metric's
//     column. ExternalAPIService.ProcessResult extracted a single metric via
//     GetMetricIndex / GetFirstConfigMetric, so the remaining columns were
//     dropped from both `schema` and `rows` with no error. Reaches
//     report_query and report_result, the two data sources whose responses
//     are built by ProcessResult.
package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// --- Bug 1: custom time range without a unit ---

// TestAccReportQueryDataSource_CustomTimeRangeWithoutUnit runs an ad-hoc query
// over an explicit date range, specifying only mode = "custom" in time_range.
// unit is left unset, which is what a user writing a custom range would
// naturally do: for a custom range the unit carries no meaning.
func TestAccReportQueryDataSource_CustomTimeRangeWithoutUnit(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryCustomTimeRangeConfig(""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("result_json"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("row_count"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestAccReportQueryDataSource_CustomTimeRangeUnitRejected covers the other
// half of the pair: supplying a unit alongside mode = "custom" is refused at
// plan time by reportCustomTimeRangeUnitValidator, so the request is never
// made. The API rejects the combination and omits unit from custom-mode
// responses; catching it here turns that 400 into an actionable error and
// avoids a configured value diffing forever against an absent one.
func TestAccReportQueryDataSource_CustomTimeRangeUnitRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryCustomTimeRangeConfig(`unit = "day"`),
				ExpectError: regexp.MustCompile(`Invalid Time Range Configuration`),
			},
		},
	})
}

// TestAccReport_CustomTimeRangeUnitRejected is the resource-side counterpart,
// confirming the validator is wired into both surfaces.
func TestAccReport_CustomTimeRangeUnitRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportCustomTimeRangeWithUnit(acctest.RandInt()),
				ExpectError: regexp.MustCompile(`unit.*must not be set when|Invalid Time Range Configuration`),
			},
		},
	})
}

// TestAccReport_CustomTimeRangeWithoutUnit covers the same missing-unit payload
// on the resource path (POST/PATCH /analytics/v1/reports) rather than the query
// path. The second step re-applies to check for drift: once the API accepts the
// request, timeRange serializes `unit` unconditionally, so a custom-mode report
// created without a unit is expected to read back as unit = "" — an
// Optional+Computed value outside the attribute's own enum. The drift step is
// what will show whether that round-trips stably.
func TestAccReport_CustomTimeRangeWithoutUnit(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportCustomTimeRangeWithoutUnit(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.custom_range",
						tfjsonpath.New("config").AtMapKey("time_range").AtMapKey("mode"),
						knownvalue.StringExact("custom")),
					statecheck.ExpectKnownValue(
						"doit_report.custom_range",
						tfjsonpath.New("config").AtMapKey("custom_time_range").AtMapKey("from"),
						knownvalue.StringExact("2026-07-27T00:00:00Z")),
				},
			},
			// Drift check.
			{
				Config: testAccReportCustomTimeRangeWithoutUnit(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// --- Bug 2: multi-metric queries drop every metric after the first ---

// TestAccReportQueryDataSource_MultiMetricReturnsAllColumns asks for two
// metrics and asserts both reach the result. The live schema for this config is
// [year, month, cost, usage, timestamp] — the trailing timestamp column is
// appended for the datetime dimensions, so the width is compared against the
// schema rather than pinned to a literal. With the bug, `usage` was absent from
// both schema and rows.
func TestAccReportQueryDataSource_MultiMetricReturnsAllColumns(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryMultiMetricConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Metric columns present, in the order requested. This is
					// the assertion that detects the dropped metric.
					resource.TestCheckOutput("query_metric_columns", "cost,usage"),
					// Guards a partial fix where schema gains a column but rows
					// do not. "no_rows" means the query returned nothing, which
					// would make this inconclusive rather than passing.
					resource.TestCheckOutput("query_row_matches_schema", "true"),
				),
			},
		},
	})
}

// TestAccReportResultDataSource_MultiMetricReturnsAllColumns covers the other
// ProcessResult caller: reading a saved two-metric report's results via
// GET /analytics/v1/reports/{id}. Only the metric column names are asserted,
// since the report config pins no dimensions and the API supplies its own —
// making the total column count unpredictable.
func TestAccReportResultDataSource_MultiMetricReturnsAllColumns(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-multimetric")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultMultiMetricConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("result_metric_columns", "cost,usage"),
				),
			},
		},
	})
}

// --- Test config helpers ---

// testAccReportQueryCustomTimeRangeConfig builds a custom-range query. extraTimeRange
// is injected into the time_range block so the same config serves both the
// unit-less case and the control that supplies a unit.
func testAccReportQueryCustomTimeRangeConfig(extraTimeRange string) string {
	return fmt.Sprintf(`
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "total"
        time_interval  = "day"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        custom_time_range = {
          from = "2026-07-27T00:00:00Z"
          to   = "2026-07-29T00:00:00Z"
        }
        time_range = {
          mode = "custom"
          %s
        }
    }
}
`, extraTimeRange)
}

// testAccReportCustomTimeRangeWithUnit is the config the validator must refuse:
// a custom range with a unit still present, as configs written against the old
// API (which required one) look today.
func testAccReportCustomTimeRangeWithUnit(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "custom_range_unit" {
    name        = "test-custom-range-with-unit-%d"
    description = "custom time range with a unit that must be rejected at plan time"
    config = {
        metrics = [{ type = "basic", value = "cost" }]
        aggregation    = "total"
        time_interval  = "day"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        custom_time_range = {
          from = "2026-07-27T00:00:00Z"
          to   = "2026-07-29T00:00:00Z"
        }
        time_range = {
          mode = "custom"
          unit = "day"
        }
    }
}
`, i)
}

func testAccReportCustomTimeRangeWithoutUnit(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "custom_range" {
    name        = "test-custom-range-no-unit-%d"
    description = "custom time range with no unit in time_range"
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "total"
        time_interval  = "day"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        custom_time_range = {
          from = "2026-07-27T00:00:00Z"
          to   = "2026-07-29T00:00:00Z"
        }
        # unit deliberately omitted — it is meaningless for a custom range.
        time_range = {
          mode = "custom"
        }
    }
}
`, i)
}

func testAccReportQueryMultiMetricConfig() string {
	return `
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          },
          {
            type  = "basic"
            value = "usage"
          }
        ]
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        time_range = {
          mode            = "last"
          amount          = 3
          unit            = "month"
          include_current = false
        }
        dimensions = [
          {
            id   = "year"
            type = "datetime"
          },
          {
            id   = "month"
            type = "datetime"
          }
        ]
    }
}

locals {
  query_result = jsondecode(data.doit_report_query.test.result_json)
}

output "query_metric_columns" {
  value = join(",", [
    for field in local.query_result.schema : field.name
    if contains(["cost", "usage"], field.name)
  ])
}

output "query_row_matches_schema" {
  value = length(local.query_result.rows) == 0 ? "no_rows" : tostring(
    length(local.query_result.rows[0]) == length(local.query_result.schema)
  )
}
`
}

func testAccReportResultMultiMetricConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "multi_metric" {
    name        = %q
    description = "two-metric report read back through report_result"
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          },
          {
            type  = "basic"
            value = "usage"
          }
        ]
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        time_range = {
          mode            = "last"
          amount          = 3
          unit            = "month"
          include_current = false
        }
    }
}

data "doit_report_result" "multi_metric" {
    id = doit_report.multi_metric.id
}

locals {
  result_payload = jsondecode(data.doit_report_result.multi_metric.result_json)
}

output "result_metric_columns" {
  value = join(",", [
    for field in local.result_payload.schema : field.name
    if contains(["cost", "usage"], field.name)
  ])
}
`, name)
}
