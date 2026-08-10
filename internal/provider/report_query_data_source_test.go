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

// TestAccReportQueryDataSource_MetricFieldsRequired verifies that report_query
// enforces the same metric-field requirement as the resource: a metric object
// missing type/value is rejected at plan time (not sent to the API as a bad
// request). report_query's config schema is derived from the report resource
// schema (convertResourceAttrsToDataSource), which now marks metric type/value
// Required, so Terraform's config decoder rejects the omission itself.
func TestAccReportQueryDataSource_MetricFieldsRequired(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceMissingMetricType(),
				ExpectError: regexp.MustCompile(`attribute "type" is required`),
			},
		},
	})
}

// TestAccReportQueryDataSource_MirrorsConflict asserts that report_query
// rejects config.metric alongside config.metrics, matching the resource. Its
// schema is derived from the generated resource schema rather than from
// reportResource.Schema, so the validator is attached in both places.
func TestAccReportQueryDataSource_MirrorsConflict(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceBothMirrors(),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

func testAccReportQueryDataSourceBothMirrors() string {
	return `
data "doit_report_query" "test" {
    config = {
        metric  = { type = "basic", value = "usage" }
        metrics = [{ type = "basic", value = "cost" }]
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
`
}

// --- Test config helpers ---

func testAccReportQueryDataSourceMissingMetricType() string {
	return `
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
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
    }
}
`
}

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

// TestAccReportQueryDataSource_FilterWithoutMode verifies that the query
// data source works when a filter omits mode (optional field).
func TestAccReportQueryDataSource_FilterWithoutMode(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceFilterNoMode(),
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

func testAccReportQueryDataSourceFilterNoMode() string {
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
        filters = [
          {
            id      = "cloud_provider"
            type    = "fixed"
            values  = ["amazon-web-services"]
          }
        ]
    }
}
`
}

func TestAccReportQueryDataSource_ForecastSettings(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceForecastSettingsConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// result_json must contain forecastRows, proving forecast_settings is
					// applied rather than silently ignored.
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("result_json"),
						knownvalue.StringRegexp(regexp.MustCompile(`forecastRows`))),
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("row_count"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestAccReportQueryDataSource_ForecastConflict verifies that forecast=false plus
// forecast_settings is accepted by the query: the API enables forecasting from the
// presence of forecast_settings and still returns forecast rows.
func TestAccReportQueryDataSource_ForecastConflict(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceForecastConflictConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("result_json"),
						knownvalue.StringRegexp(regexp.MustCompile(`forecastRows`))),
				},
			},
		},
	})
}

// TestAccReportQueryDataSource_EmptyForecastRange verifies an empty forecast custom
// date range is rejected at plan time (matching the report resource) rather than
// only failing at the API.
func TestAccReportQueryDataSource_EmptyForecastRange(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceEmptyForecastRangeConfig(),
				ExpectError: regexp.MustCompile(`Empty Future Custom Date Range`),
			},
		},
	})
}

func testAccReportQueryDataSourceForecastSettingsConfig() string {
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
        forecast_settings = {
            future_time_intervals     = 3
            historical_time_intervals = 12
            mode                      = "totals"
        }
    }
}
`
}

func testAccReportQueryDataSourceForecastConflictConfig() string {
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
        advanced_analysis = {
            forecast = false
        }
        forecast_settings = {
            future_time_intervals = 3
            mode                  = "totals"
        }
    }
}
`
}

func testAccReportQueryDataSourceEmptyForecastRangeConfig() string {
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
        forecast_settings = {
            future_custom_date_range = {}
            mode                     = "totals"
        }
    }
}
`
}

// TestAccReportQueryDataSource_Count verifies an ad-hoc count-aggregation query
// with a count field executes and returns results.
func TestAccReportQueryDataSource_Count(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceCountConfig(),
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

// TestAccReportQueryDataSource_CountInvalidAggregation verifies the shared
// count/aggregation validator also fires for the query data source.
func TestAccReportQueryDataSource_CountInvalidAggregation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceCountInvalidConfig(),
				ExpectError: regexp.MustCompile(`Invalid Count Configuration`),
			},
		},
	})
}

func testAccReportQueryDataSourceCountConfig() string {
	return `
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "count"
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
        count = {
            id   = "service_description"
            type = "fixed"
        }
    }
}
`
}

func testAccReportQueryDataSourceCountInvalidConfig() string {
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
        count = {
            id   = "service_description"
            type = "fixed"
        }
    }
}
`
}

// TestAccReportQueryDataSource_CountNoAggregation verifies the shared
// count/aggregation validator rejects count with an omitted aggregation for the
// query data source too (aggregation does not default to "count").
func TestAccReportQueryDataSource_CountNoAggregation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceCountNoAggregationConfig(),
				ExpectError: regexp.MustCompile(`Invalid Count Configuration`),
			},
		},
	})
}

func testAccReportQueryDataSourceCountNoAggregationConfig() string {
	return `
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
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
        count = {
            id   = "service_description"
            type = "fixed"
        }
    }
}
`
}

// TestAccReportQueryDataSource_CountRequiredWhenAggregationCount verifies the
// shared validator also rejects aggregation = "count" without a count block for
// the query data source.
func TestAccReportQueryDataSource_CountRequiredWhenAggregationCount(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceCountAggNoBlockConfig(),
				ExpectError: regexp.MustCompile(`Missing Count Configuration`),
			},
		},
	})
}

func testAccReportQueryDataSourceCountAggNoBlockConfig() string {
	return `
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "count"
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
`
}
