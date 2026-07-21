package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccReport_ForecastSettings_Dynamic(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportDynamicForecast(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("doit_report.forecast_test", "config.advanced_analysis.forecast", "true"),
					resource.TestCheckResourceAttr("doit_report.forecast_test", "config.forecast_settings.mode", "totals"),
				),
			},
		},
	})
}

func TestAccReport_ForecastSettings_Import(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithForecastEnabledButSettingsOmitted(n),
			},
			{
				ResourceName:      "doit_report.forecast_import_test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccReport_ForecastSettings_IntervalsToCustomTransition(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithIntervals(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("doit_report.intervals_test", "config.forecast_settings.future_time_intervals", "12"),
					resource.TestCheckNoResourceAttr("doit_report.intervals_test", "config.forecast_settings.future_custom_date_range"),
				),
			},
			{
				Config: testAccReportWithCustomDates(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("doit_report.intervals_test", "config.forecast_settings.future_time_intervals"),
					resource.TestCheckResourceAttrSet("doit_report.intervals_test", "config.forecast_settings.future_custom_date_range.from"),
				),
			},
			{
				Config: testAccReportWithIntervals(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("doit_report.intervals_test", "config.forecast_settings.future_time_intervals", "12"),
					resource.TestCheckNoResourceAttr("doit_report.intervals_test", "config.forecast_settings.future_custom_date_range"),
				),
			},
		},
	})
}

func testAccReportDynamicForecast(n int) string {
	return fmt.Sprintf(`
resource "doit_report" "dep" {
  name        = "dep-%d"
  description = "dependency report"
}

resource "doit_report" "forecast_test" {
  name        = "test-forecast-dynamic-%d"
  description = "report testing dynamic unknown forecast"
  config = {
    advanced_analysis = {
      forecast = doit_report.dep.id != ""
    }
  }
}
`, n, n)
}

func testAccReportWithForecastEnabledButSettingsOmitted(n int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_import_test" {
  name        = "test-forecast-import-%d"
  description = "report testing import drift"
  config = {
    advanced_analysis = {
      forecast = true
    }
  }
}
`, n)
}

func testAccReportWithIntervals(n int) string {
	return fmt.Sprintf(`
resource "doit_report" "intervals_test" {
  name        = "test-intervals-transition-%d"
  description = "report testing intervals transition"
  config = {
    advanced_analysis = {
      forecast = true
    }
    forecast_settings = {
      future_time_intervals = 12
      mode                  = "totals"
    }
  }
}
`, n)
}

func testAccReportWithCustomDates(n int) string {
	return fmt.Sprintf(`
resource "doit_report" "intervals_test" {
  name        = "test-intervals-transition-%d"
  description = "report testing intervals transition"
  config = {
    advanced_analysis = {
      forecast = true
    }
    forecast_settings = {
      future_custom_date_range = {
        from = "2026-08-01T00:00:00Z"
        to   = "2026-09-01T00:00:00Z"
      }
      mode = "totals"
    }
  }
}
`, n)
}
