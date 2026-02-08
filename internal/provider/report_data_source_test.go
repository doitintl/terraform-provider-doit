package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccReportDataSource(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source attributes match resource
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-ds-report-%d", n))),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("test report for data source")),
					// Verify config object is populated
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("aggregation"),
						knownvalue.StringExact("total")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("currency"),
						knownvalue.StringExact("USD")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("time_interval"),
						knownvalue.StringExact("month")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("layout"),
						knownvalue.StringExact("table")),
				},
			},
		},
	})
}

func TestAccReportDataSource_WithFilters(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceWithFiltersConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify filters are populated in data source
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListSizeExact(1)),
					// Verify group is populated
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("group"),
						knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

func testAccReportDataSourceConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = "test-ds-report-%d"
    description = "test report for data source"
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
    }
}

data "doit_report" "test" {
    id = doit_report.test.id
}
`, i)
}

func testAccReportDataSourceWithFiltersConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = "test-ds-report-filters-%d"
    description = "test report with filters for data source"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        data_source   = "billing"
        currency      = "USD"
        layout        = "table"
        display_values = "actuals_only"
        filters = [
          {
            id      = "cloud_provider"
            type    = "fixed"
            inverse = false
            values  = ["google-cloud"]
            mode    = "is"
          }
        ]
        group = [
          {
            id   = "service_description"
            type = "fixed"
          }
        ]
    }
}

data "doit_report" "test" {
    id = doit_report.test.id
}
`, i)
}

func TestAccReportDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(not found|404|Not Found)`),
			},
		},
	})
}

func testAccReportDataSourceNotFoundConfig() string {
	return `
data "doit_report" "test" {
    id = "non-existent-report-id"
}
`
}
