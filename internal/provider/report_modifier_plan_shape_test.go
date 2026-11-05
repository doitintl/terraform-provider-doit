package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccReport_PlanShapeOfForecastSettings pins the plan that
// useNullOrDefaultForForecastSettings sees and produces, so the unit table in
// planmodifier_decision_table_internal_test.go (TestUseNullOrDefaultForForecastSettings)
// builds its inputs from what Terraform really passes:
//
//   - forecast = true, forecast_settings omitted: the modifier plans the default
//     object (mode "totals"), on create and on an update of another attribute
//   - forecast_settings configured: the plan keeps the configured object
//   - forecast = false, forecast_settings omitted: null, also on an update
//   - advanced_analysis omitted: null
func TestAccReport_PlanShapeOfForecastSettings(t *testing.T) {
	n := acctest.RandInt()
	config := func(description, advancedAnalysis, forecastSettings string) string {
		return fmt.Sprintf(`
resource "doit_report" "test" {
  name        = "test-plan-shape-forecast-%d"
  description = %q
  config = {
    %s
    %s
  }
}
`, n, description, advancedAnalysis, forecastSettings)
	}
	const (
		forecastOn  = `advanced_analysis = { forecast = true }`
		forecastOff = `advanced_analysis = { forecast = false }`
		settings    = `forecast_settings = { future_time_intervals = 12, mode = "totals" }`
	)
	const resourceName = "doit_report.test"
	settingsPath := tfjsonpath.New("config").AtMapKey("forecast_settings")
	checkPlan := func(checks ...plancheck.PlanCheck) resource.ConfigPlanChecks {
		return resource.ConfigPlanChecks{PreApply: checks}
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Create: omitted settings plan as the default object, not unknown.
				Config: config("one", forecastOn, ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("mode"), knownvalue.StringExact("totals")),
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("future_time_intervals"), knownvalue.Null()),
				),
			},
			{
				// Update of another attribute: the default object, still not unknown.
				Config: config("two", forecastOn, ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("mode"), knownvalue.StringExact("totals")),
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("future_time_intervals"), knownvalue.Null()),
				),
			},
			{
				// Configured settings are planned as configured.
				Config: config("two", forecastOn, settings),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("mode"), knownvalue.StringExact("totals")),
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("future_time_intervals"), knownvalue.Int64Exact(12)),
				),
			},
			{
				// Settings dropped from the config while forecast stays on: the plan
				// holds the stored settings, and the modifier resets them to the
				// default object rather than keeping them.
				Config: config("two", forecastOn, ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("mode"), knownvalue.StringExact("totals")),
					plancheck.ExpectKnownValue(resourceName, settingsPath.AtMapKey("future_time_intervals"), knownvalue.Null()),
				),
			},
			{
				// ... and that is stable: nothing else changes, the plan is empty.
				Config:           config("two", forecastOn, ""),
				ConfigPlanChecks: checkPlan(plancheck.ExpectEmptyPlan()),
			},
			{
				// Forecast switched off with the settings omitted: null.
				Config: config("two", forecastOff, ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath, knownvalue.Null()),
				),
			},
			{
				// Update of another attribute with forecast off: still null.
				Config: config("three", forecastOff, ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath, knownvalue.Null()),
				),
			},
			{
				// advanced_analysis omitted after forecast was false: null. The plan
				// keeps the stored forecast (a known false, not null), but the
				// modifier decides on the config, where it is null.
				Config: config("three", "", ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, settingsPath, knownvalue.Null()),
					plancheck.ExpectKnownValue(resourceName, tfjsonpath.New("config").AtMapKey("advanced_analysis").AtMapKey("forecast"), knownvalue.Bool(false)),
				),
			},
		},
	})
}

// TestAccReport_PlanShapeOfOmittedMetrics pins the plan that
// useEmptyForUnconfiguredMetricsMirror sees and produces, so the unit table
// (TestUseEmptyForUnconfiguredMetricsMirror) builds its inputs from what
// Terraform really passes: omitted config.metrics plans as an empty list, on
// create and on an update of another attribute, and configured metrics are
// planned as configured.
func TestAccReport_PlanShapeOfOmittedMetrics(t *testing.T) {
	n := acctest.RandInt()
	config := func(description, metrics string) string {
		return fmt.Sprintf(`
resource "doit_report" "test" {
  name        = "test-plan-shape-metrics-%d"
  description = %q
  config = {
    currency = "USD"
    %s
  }
}
`, n, description, metrics)
	}
	const resourceName = "doit_report.test"
	metricsPath := tfjsonpath.New("config").AtMapKey("metrics")
	empty := knownvalue.ListExact([]knownvalue.Check{})
	checkPlan := func(checks ...plancheck.PlanCheck) resource.ConfigPlanChecks {
		return resource.ConfigPlanChecks{PreApply: checks}
	}
	const configured = `metrics = [{ type = "basic", value = "cost" }]`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Create: omitted metrics plan as [], not unknown.
				Config: config("one", ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, metricsPath, empty),
				),
			},
			{
				// Update of another attribute: [] again.
				Config: config("two", ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, metricsPath, empty),
				),
			},
			{
				// Nothing changes: the plan is empty.
				Config:           config("two", ""),
				ConfigPlanChecks: checkPlan(plancheck.ExpectEmptyPlan()),
			},
			{
				// Configured metrics are planned as configured.
				Config: config("two", configured),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, metricsPath, knownvalue.ListSizeExact(1)),
					plancheck.ExpectKnownValue(resourceName, metricsPath.AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("cost")),
				),
			},
			{
				// Removed again: back to [].
				Config: config("two", ""),
				ConfigPlanChecks: checkPlan(
					plancheck.ExpectKnownValue(resourceName, metricsPath, empty),
				),
			},
		},
	})
}

// TestAccReport_ForecastSettingsWhenAdvancedAnalysisOmittedAfterForecastOn
// probes an update that drops advanced_analysis from the config of a report
// stored with forecast = true and forecast settings.
func TestAccReport_ForecastSettingsWhenAdvancedAnalysisOmittedAfterForecastOn(t *testing.T) {
	n := acctest.RandInt()
	config := func(advancedAnalysis string) string {
		return fmt.Sprintf(`
resource "doit_report" "test" {
  name        = "test-forecast-omitted-after-on-%d"
  description = "probe"
  config = {
    %s
  }
}
`, n, advancedAnalysis)
	}
	const resourceName = "doit_report.test"
	configPath := tfjsonpath.New("config")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config(`advanced_analysis = { forecast = true }`)},
			{
				Config: config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(resourceName, configPath.AtMapKey("advanced_analysis").AtMapKey("forecast"), knownvalue.Bool(true)),
					plancheck.ExpectKnownValue(resourceName, configPath.AtMapKey("forecast_settings"), knownvalue.Null()),
				}},
			},
			{
				Config:           config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
