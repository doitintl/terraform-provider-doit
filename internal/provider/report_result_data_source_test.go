package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccReportResultDataSource verifies that a report's results can be
// fetched as a JSON string. It creates a report resource, then reads
// its results via the data source.
func TestAccReportResultDataSource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify result_json is populated (non-empty)
					statecheck.ExpectKnownValue(
						"data.doit_report_result.test",
						tfjsonpath.New("result_json"),
						knownvalue.NotNull()),
					// Verify report_name matches
					statecheck.ExpectKnownValue(
						"data.doit_report_result.test",
						tfjsonpath.New("report_name"),
						knownvalue.StringExact(rName)),
					// Verify cache_hit is known (either true or false)
					statecheck.ExpectKnownValue(
						"data.doit_report_result.test",
						tfjsonpath.New("cache_hit"),
						knownvalue.NotNull()),
					// Verify row_count is known
					statecheck.ExpectKnownValue(
						"data.doit_report_result.test",
						tfjsonpath.New("row_count"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestAccReportResultDataSource_WithDateRange verifies the start_date/end_date
// time range override parameters.
func TestAccReportResultDataSource_WithDateRange(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds-dr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultDataSourceWithDateRangeConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_report_result.test",
						tfjsonpath.New("result_json"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"data.doit_report_result.test",
						tfjsonpath.New("report_name"),
						knownvalue.StringExact(rName)),
				},
			},
		},
	})
}

// TestAccReportResultDataSource_ReApply verifies that re-applying the same
// config does not produce unexpected errors. Report results may or may not
// change between runs depending on caching and data ingestion timing.
func TestAccReportResultDataSource_ReApply(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds-ra")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultDataSourceConfig(rName),
			},
			// Re-apply the same config — should succeed regardless of
			// whether data has changed (empty or non-empty plan is fine).
			{
				Config: testAccReportResultDataSourceConfig(rName),
			},
		},
	})
}

// TestAccReportResultDataSource_NotFound verifies proper error handling
// when fetching results for a non-existent report.
func TestAccReportResultDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportResultDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(not found|404|Not Found)`),
			},
		},
	})
}

// --- Test config helpers ---

func testAccReportResultDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
    description = "test report for result data source"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        time_range = {
          mode           = "last"
          amount         = 3
          unit           = "month"
          include_current = false
        }
    }
}

data "doit_report_result" "test" {
    id = doit_report.test.id
}
`, name)
}

func testAccReportResultDataSourceWithDateRangeConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
    description = "test report for result data source with date range"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        time_range = {
          mode           = "last"
          amount         = 3
          unit           = "month"
          include_current = false
        }
    }
}

data "doit_report_result" "test" {
    id         = doit_report.test.id
    start_date = "2026-01-01"
    end_date   = "2026-01-31"
}
`, name)
}

func testAccReportResultDataSourceNotFoundConfig() string {
	return `
data "doit_report_result" "test" {
    id = "non-existent-report-id"
}
`
}
