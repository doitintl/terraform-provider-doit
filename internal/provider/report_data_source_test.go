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

func TestAccReportDataSource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-report-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source attributes match resource
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
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
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccReportDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccReportDataSource_WithFilters(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-report-ds-f")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceWithFiltersConfig(rName),
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

func testAccReportDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
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
`, name)
}

func testAccReportDataSourceWithFiltersConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
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
`, name)
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

// TestAccReportDataSource_FolderId verifies that the data source returns the
// correct folder_id when a report is created inside a folder.
func TestAccReportDataSource_FolderId(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-report-ds-f")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceFolderIdConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_report.test", "folder_id",
						"doit_folder.ds_test", "id"),
				),
			},
		},
	})
}

func testAccReportDataSourceFolderIdConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "ds_test" {
    name = "%s-folder"
}

resource "doit_report" "test" {
    name      = %q
    folder_id = doit_folder.ds_test.id
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
`, name, name)
}

func TestAccReportDataSource_ForecastSettings(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-report-ds-fs")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceForecastSettingsConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify forecast_settings are populated in data source
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_time_intervals"),
						knownvalue.Int64Exact(3)),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_time_intervals"),
						knownvalue.Int64Exact(12)),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("mode"),
						knownvalue.StringExact("totals")),
				},
			},
		},
	})
}

func testAccReportDataSourceForecastSettingsConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
    description = "test report with forecast settings for data source"
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
        forecast_settings = {
            future_time_intervals     = 3
            historical_time_intervals = 12
            mode                      = "totals"
        }
    }
}

data "doit_report" "test" {
    id = doit_report.test.id
}
`, name)
}

func TestAccReportDataSource_ForecastSettingsCustomDates(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-report-ds-fscd")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceForecastSettingsCustomDatesConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify forecast_settings custom date ranges are populated in data source
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range").AtMapKey("from"),
						knownvalue.StringExact("2026-07-15T00:00:00Z")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range").AtMapKey("to"),
						knownvalue.StringExact("2026-10-15T00:00:00Z")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range").AtMapKey("from"),
						knownvalue.StringExact("2025-07-15T00:00:00Z")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range").AtMapKey("to"),
						knownvalue.StringExact("2026-07-15T00:00:00Z")),
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("mode"),
						knownvalue.StringExact("totals")),
				},
			},
		},
	})
}

func testAccReportDataSourceForecastSettingsCustomDatesConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
    description = "test report with forecast settings custom dates for data source"
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
        forecast_settings = {
            future_custom_date_range = {
                from = "2026-07-15T00:00:00Z"
                to   = "2026-10-15T00:00:00Z"
            }
            historical_custom_date_range = {
                from = "2025-07-15T00:00:00Z"
                to   = "2026-07-15T00:00:00Z"
            }
            mode = "totals"
        }
    }
}

data "doit_report" "test" {
    id = doit_report.test.id
}
`, name)
}

func TestAccReportDataSource_ForecastSettings_Omitted(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-report-ds-fso")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDataSourceForecastSettingsOmittedConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify forecast_settings is Null in data source state
					statecheck.ExpectKnownValue(
						"data.doit_report.test",
						tfjsonpath.New("config").AtMapKey("forecast_settings"),
						knownvalue.Null()),
				},
			},
		},
	})
}

func testAccReportDataSourceForecastSettingsOmittedConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
    name        = %q
    description = "test report with omitted forecast settings for data source"
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
`, name)
}
