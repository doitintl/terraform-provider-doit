// Acceptance coverage for two behaviours of the external Cloud Analytics API:
// custom time ranges, which carry no `unit`, and multi-metric queries, which
// must return one column per requested metric.
//
// Each spans two provider surfaces — custom ranges reach the report resource
// and doit_report_query; multi-metric results reach doit_report_query and
// doit_report_result — so both are asserted on each surface.
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

// --- Custom time ranges ---

// TestAccReportQueryDataSource_CustomTimeRangeWithoutUnit asserts an ad-hoc
// query over an explicit date range succeeds with only mode = "custom" set, no
// unit configured.
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

// TestAccReportQueryDataSource_CustomTimeRangeUnitRejected asserts
// reportCustomTimeRangeUnitValidator refuses a unit alongside mode = "custom"
// at plan time, so no request is made.
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

// TestAccReport_CustomTimeRangeUnitRejected asserts the same rejection on the
// resource, confirming the validator is wired into both surfaces.
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

// TestAccReport_CustomTimeRangeWithoutUnit asserts a report with a custom range
// and no unit is created, that the unit the API omits from its response reads
// back as null, and that the pair round-trips without drift.
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
					// The API omits unit for a custom range; the mapper records
					// the absent field as null rather than dereferencing it.
					statecheck.ExpectKnownValue(
						"doit_report.custom_range",
						tfjsonpath.New("config").AtMapKey("time_range").AtMapKey("unit"),
						knownvalue.Null()),
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

// --- Multi-metric queries ---

// TestAccReportQueryDataSource_MultiMetricReturnsAllColumns asserts every
// requested metric reaches the result. The schema for this config is
// [year, month, cost, usage, timestamp], the trailing timestamp column being
// appended for the datetime dimensions, so row width is compared against the
// schema rather than a literal.
func TestAccReportQueryDataSource_MultiMetricReturnsAllColumns(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryMultiMetricConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Both metric columns present, in the order requested.
					resource.TestCheckOutput("query_metric_columns", "cost,usage"),
					// Row width matches the schema, so a schema column without a
					// corresponding cell fails. "no_rows" marks the check
					// inconclusive rather than passing it.
					resource.TestCheckOutput("query_row_matches_schema", "true"),
				),
			},
		},
	})
}

// TestAccReportResultDataSource_MultiMetricReturnsAllColumns asserts the same
// for a saved report read through doit_report_result. Only the metric column
// names are checked: the config pins no dimensions, so the API supplies its own
// and the total column count is not predictable.
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

// testAccReportQueryCustomTimeRangeConfig builds a custom-range query.
// extraTimeRange is injected into the time_range block so one config serves both
// the unit-less and unit-present cases.
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
// a custom range with a unit set.
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
