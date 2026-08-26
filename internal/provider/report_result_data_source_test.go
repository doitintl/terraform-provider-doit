package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// TestAccReportResultDataSource_WithTimeRange verifies the time_range
// ISO 8601 duration override parameter.
func TestAccReportResultDataSource_WithTimeRange(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds-tr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultDataSourceWithTimeRangeConfig(rName),
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

func testAccReportResultDataSourceWithTimeRangeConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
    description = "test report for result data source with time range"
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
    time_range = "P1M"
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

// TestAccReportResultDataSource_AllocationSchemaFieldID verifies that when a saved
// report is configured with an allocation dimension, reading its results via the
// data source returns the allocation's id in the schema metadata.
func TestAccReportResultDataSource_AllocationSchemaFieldID(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-rr-alloc")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultDataSourceAllocationConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_allocation.group", "id"),
					resource.TestCheckOutput("schema_id_matches_allocation", "true"),
					func(s *terraform.State) error {
						allocRes, ok := s.RootModule().Resources["doit_allocation.group"]
						if !ok {
							return fmt.Errorf("resource doit_allocation.group not found")
						}
						schemaID := s.RootModule().Outputs["allocation_schema_id"].Value.(string)
						if schemaID != allocRes.Primary.ID {
							return fmt.Errorf("expected schema id %q, got %q", allocRes.Primary.ID, schemaID)
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccReportResultDataSourceAllocationConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "group" {
    name              = "%[1]s-group"
    description       = "test allocation group"
    unallocated_costs = "%[1]s-other"
    rules = [
        {
            action  = "create"
            name    = "%[1]s-rule"
            formula = "A"
            components = [
                {
                    key    = "country"
                    mode   = "is"
                    type   = "fixed"
                    values = ["US"]
                }
            ]
        }
    ]
}

resource "doit_report" "test" {
    name        = "%[1]s-report"
    description = "test report with allocation dimension read through report_result"
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
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
        group = [
          {
            id   = doit_allocation.group.id
            type = "allocation"
          }
        ]
    }
}

data "doit_report_result" "test" {
    id = doit_report.test.id
}

locals {
  result_payload = jsondecode(data.doit_report_result.test.result_json)
  allocation_fields = [
    for field in local.result_payload.schema : field
    if lookup(field, "id", "") == doit_allocation.group.id
  ]
}

output "allocation_schema_id" {
  value = length(local.allocation_fields) > 0 ? lookup(local.allocation_fields[0], "id", "") : ""
}

output "schema_id_matches_allocation" {
  value = tostring(length(local.allocation_fields) > 0 && lookup(local.allocation_fields[0], "id", "") == doit_allocation.group.id)
}
`, name)
}

// TestAccReportResultDataSource_SchemaFieldMetadata verifies that reading a report's
// results returns column metadata (name, type, unit, currency, aggregation) in result_json.schema.
func TestAccReportResultDataSource_SchemaFieldMetadata(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-rr-meta")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportResultDataSourceSchemaFieldMetadataConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("cost_field_name", "cost"),
					resource.TestCheckOutput("cost_field_type", "float"),
					resource.TestCheckOutput("cost_field_unit", "currency"),
					resource.TestCheckOutput("cost_field_currency", "USD"),
					resource.TestCheckOutput("cost_field_aggregation", "total"),
				),
			},
		},
	})
}

func testAccReportResultDataSourceSchemaFieldMetadataConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = "%[1]s-report"
    description = "test report schema field metadata"
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
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
          amount          = 1
          unit            = "month"
          include_current = false
        }
    }
}

data "doit_report_result" "test" {
    id = doit_report.test.id
}

locals {
  result_payload = jsondecode(data.doit_report_result.test.result_json)
  cost_fields = [
    for field in local.result_payload.schema : field
    if lookup(field, "name", "") == "cost"
  ]
  cost_field = length(local.cost_fields) > 0 ? local.cost_fields[0] : {}
}

output "cost_field_name" {
  value = lookup(local.cost_field, "name", "")
}

output "cost_field_type" {
  value = lookup(local.cost_field, "type", "")
}

output "cost_field_unit" {
  value = lookup(local.cost_field, "unit", "")
}

output "cost_field_currency" {
  value = lookup(local.cost_field, "currency", "")
}

output "cost_field_aggregation" {
  value = lookup(local.cost_field, "aggregation", "")
}
`, name)
}
