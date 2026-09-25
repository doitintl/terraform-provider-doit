package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestAccReportQueryDataSource_FileOutput(t *testing.T) {
	config := strings.Replace(testAccReportQueryDataSourceConfig(),
		"data \"doit_report_query\" \"test\" {", "data \"doit_report_query\" \"test\" {\n    file_output = \"png\"", 1)
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: config,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.doit_report_query.test", tfjsonpath.New("file_output_url"), knownvalue.StringRegexp(regexp.MustCompile(`^https://`))),
				statecheck.ExpectKnownValue("data.doit_report_query.test", tfjsonpath.New("result_json"), knownvalue.NotNull()),
			},
		}},
	})
}

func TestAccReportQueryDataSource_InvalidFileOutput(t *testing.T) {
	config := strings.Replace(testAccReportQueryDataSourceConfig(),
		"data \"doit_report_query\" \"test\" {", "data \"doit_report_query\" \"test\" {\n    file_output = \"csv\"", 1)
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexp.MustCompile(`(?i)file_output|one of`),
		}},
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

// TestAccReportQueryDataSource_LimitByChange_InvalidOperator verifies that
// report_query inherits the report resource schema's nested attribute validator
// (via convertResourceAttrsToDataSource) and rejects deprecated SQL-style
// operators (e.g. ">=", "between") at plan time.
func TestAccReportQueryDataSource_LimitByChange_InvalidOperator(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportQueryDataSourceLimitByChangeOperator(">="),
				ExpectError: regexp.MustCompile(`Attribute config\.limit_by_change\.operator value must be one of:`),
			},
			{
				Config:      testAccReportQueryDataSourceLimitByChangeOperator("between"),
				ExpectError: regexp.MustCompile(`Attribute config\.limit_by_change\.operator value must be one of:`),
			},
		},
	})
}

func testAccReportQueryDataSourceLimitByChangeOperator(op string) string {
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
        limit_by_change = {
          metric = {
            type  = "basic"
            value = "cost"
          }
          change_type             = "percentage"
          operator                = %q
          values                  = [50]
          include_incomplete_data = false
        }
    }
}
`, op)
}

// TestAccReportQueryDataSource_WithTimeout verifies the timeouts block is wired
// up and a generous read timeout is honored. Queries run asynchronously, so this
// timeout bounds the whole poll loop — exceeding it cancels the operation. The
// expiry path itself is covered deterministically by the unit tests in
// async_report_test.go.
func TestAccReportQueryDataSource_WithTimeout(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceConfigWithTimeout(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_report_query.with_timeout",
						tfjsonpath.New("result_json"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"data.doit_report_query.with_timeout",
						tfjsonpath.New("row_count"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}

func testAccReportQueryDataSourceConfigWithTimeout() string {
	return `
data "doit_report_query" "with_timeout" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation    = "total"
        time_interval  = "month"
        time_range = {
          mode            = "last"
          amount          = 1
          unit            = "month"
          include_current = false
        }
    }

    timeouts = {
      read = "10m"
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

// TestAccReportQueryDataSource_AllocationSchemaFieldID verifies that when a query
// is configured with an allocation dimension, the returned schema contains the
// allocation's id.
func TestAccReportQueryDataSource_AllocationSchemaFieldID(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-rq-alloc")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceAllocationConfig(rName),
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

func testAccReportQueryDataSourceAllocationConfig(name string) string {
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
        group = [
          {
            id   = doit_allocation.group.id
            type = "allocation"
          }
        ]
    }
}

locals {
  query_result = jsondecode(data.doit_report_query.test.result_json)
  allocation_fields = [
    for field in local.query_result.schema : field
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

// TestAccReportQueryDataSource_SchemaFieldMetadata verifies that executing a query
// returns column metadata (name, type, unit, currency, aggregation) in result_json.schema.
func TestAccReportQueryDataSource_SchemaFieldMetadata(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceSchemaFieldMetadataConfig(),
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

func testAccReportQueryDataSourceSchemaFieldMetadataConfig() string {
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
          amount          = 1
          unit            = "month"
          include_current = false
        }
    }
}

locals {
  query_result = jsondecode(data.doit_report_query.test.result_json)
  cost_fields = [
    for field in local.query_result.schema : field
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
`
}

// TestAccReportQueryDataSource_DetailsValueAliases verifies that when a DataHub
// dataset has been renamed, report query results include the details object with
// valueAliases mapping the dimension ID and dataset name to the display name.
func TestAccReportQueryDataSource_DetailsValueAliases(t *testing.T) {
	dataset := os.Getenv("TEST_DATAHUB_DATASET")
	if dataset == "" {
		t.Skip("TEST_DATAHUB_DATASET environment variable not set")
	}

	client := getAPIClient(t)

	// Fetch current dataset so we can ensure expectedDisplayName is set
	origResp, err := client.GetDatahubDatasetWithResponse(t.Context(), dataset)
	if err != nil {
		t.Fatalf("Failed to fetch %s dataset: %v", dataset, err)
	}
	if origResp.StatusCode() != 200 || origResp.JSON200 == nil {
		t.Fatalf("%s dataset not found (status %d): %s", dataset, origResp.StatusCode(), string(origResp.Body))
	}
	expectedDisplayName := "Test Manual Display"
	if origResp.JSON200.DisplayName != nil && *origResp.JSON200.DisplayName != "" {
		expectedDisplayName = *origResp.JSON200.DisplayName
	} else {
		patchResp, err := client.UpdateDatahubDatasetWithResponse(t.Context(), dataset, models.UpdateDatahubDatasetJSONRequestBody{
			DisplayName: &expectedDisplayName,
		})
		if err != nil {
			t.Fatalf("Failed to update %s display name: %v", dataset, err)
		}
		if patchResp.StatusCode() != 200 {
			t.Fatalf("Update %s failed with status %d: %s", dataset, patchResp.StatusCode(), string(patchResp.Body))
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportQueryDataSourceConfigDetails(dataset),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_report_query.test",
						tfjsonpath.New("result_json"),
						knownvalue.NotNull()),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("dataset_alias", expectedDisplayName),
				),
			},
		},
	})
}

func testAccReportQueryDataSourceConfigDetails(dataset string) string {
	return fmt.Sprintf(`
data "doit_report_query" "test" {
    config = {
        metrics = [
          {
            type  = "basic"
            value = "cost"
          }
        ]
        aggregation   = "total"
        time_interval = "month"
        data_source   = "billing"
        custom_time_range = {
          from = "2026-03-01T00:00:00Z"
          to   = "2026-03-31T23:59:59Z"
        }
        time_range = {
          mode = "custom"
        }
        dimensions = [
          {
            id   = "cloud_provider"
            type = "fixed"
          }
        ]
    }
}

locals {
  query_result  = jsondecode(data.doit_report_query.test.result_json)
  details       = lookup(local.query_result, "details", {})
  value_aliases = lookup(local.details, "valueAliases", {})
  cloud_aliases = lookup(local.value_aliases, "fixed:cloud_provider", {})
}

output "dataset_alias" {
  value = lookup(local.cloud_aliases, %q, "")
}
`, dataset)
}
