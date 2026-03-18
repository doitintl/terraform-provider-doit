package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccReportQueryDataSource verifies that an ad-hoc query can be executed
// and results are returned as JSON.
func TestAccReportQueryDataSource(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("result_json"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("cache_hit"),
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

// TestAccReportQueryDataSource_ReApply verifies that re-applying the same
// config does not produce unexpected errors.
func TestAccReportQueryDataSource_ReApply(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceConfig(),
			},
			{
				Config: testAccReportQueryDataSourceConfig(),
			},
		},
	})
}

// TestAccReportQueryDataSource_InvalidConfig verifies that an invalid config
// produces a meaningful error from the API.
func TestAccReportQueryDataSource_InvalidConfig(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceInvalidConfig(),
				ExpectError: regexp.MustCompile(`(?i)Could not execute report query|invalid|secondaryTimeRange`),
			},
		},
	})
}

// --- Test config helpers ---

func testAccReportQueryDataSourceConfig() string {
	return `
data "doit_report_query" "test" {
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
`
}

func testAccReportQueryDataSourceInvalidConfig() string {
	return `
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "total"
        time_interval  = "month"
        currency       = "USD"
        time_range = {
          mode            = "last"
          amount          = 12
          unit            = "month"
          include_current = true
        }
        secondary_time_range = {
          amount          = 12
          unit            = "month"
          include_current = false
        }
    }
}
`
}

// TestAccReportQueryDataSource_WithComputedInput verifies that the data source
// correctly defers execution when config contains unknown values (computed from
// another resource). During plan, the computed value is unknown and the query
// should return unknown outputs. During apply, the value resolves and the query
// executes successfully.
func TestAccReportQueryDataSource_WithComputedInput(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceWithComputedInputConfig(),
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

func testAccReportQueryDataSourceWithComputedInputConfig() string {
	return `
# terraform_data provides a computed output that is unknown during plan.
# We use it to inject "USD" as a computed value for the currency field.
resource "terraform_data" "currency" {
    input = "USD"
}

data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "total"
        time_interval  = "month"
        currency       = terraform_data.currency.output
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
`
}
