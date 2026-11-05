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

// TestAccReport_PlanShapeOfOmittedOptionalComputed pins what the plan holds for
// an Optional+Computed attribute the config omits. The implement-resource skill
// ("Guards that read an omitted Optional+Computed attribute in ModifyPlan")
// builds on these shapes, so a framework change that alters one should fail here.
//
//   - nothing else changes: the stored value stays, the plan is empty
//   - another attribute changes: no Default and no modifier -> unknown
//     (top-level labels, nested config.data_source)
//   - another attribute changes: schema Default -> the default (folder_id)
//   - another attribute changes, clearing modifier (config.filters[*].mode) ->
//     the modifier's value (null), which is a deliberate clear, not "unknown"
func TestAccReport_PlanShapeOfOmittedOptionalComputed(t *testing.T) {
	n := acctest.RandInt()
	config := func(currency, mode string) string {
		return fmt.Sprintf(`
resource "doit_report" "test" {
  name = "test-plan-shape-%d"
  config = {
    currency      = %q
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    metrics = [{ type = "basic", value = "cost" }]
    filters = [{
      id     = "cloud_provider"
      type   = "fixed"
      values = ["google-cloud"]
      %s
    }]
  }
}
`, n, currency, mode)
	}
	const withMode = `mode = "is"`
	const resourceName = "doit_report.test"
	configPath := tfjsonpath.New("config")
	modePath := configPath.AtMapKey("filters").AtSliceIndex(0).AtMapKey("mode")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config("USD", withMode)},
			{
				// Nothing changes: omitted attributes keep their stored values.
				Config:   config("USD", withMode),
				PlanOnly: true,
			},
			{
				// Another attribute changes.
				Config: config("EUR", withMode),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue(resourceName, tfjsonpath.New("labels")),
					plancheck.ExpectUnknownValue(resourceName, configPath.AtMapKey("data_source")),
					plancheck.ExpectUnknownValue(resourceName, configPath.AtMapKey("secondary_time_range")),
					plancheck.ExpectUnknownValue(resourceName, configPath.AtMapKey("aggregation")),
					plancheck.ExpectKnownValue(resourceName, tfjsonpath.New("folder_id"), knownvalue.StringExact("root")),
				}},
			},
			{
				// A clearing modifier plans null, not the stored value and not unknown.
				Config: config("USD", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(resourceName, modePath, knownvalue.Null()),
				}},
			},
		},
	})
}

// TestAccReport_CumulativeComparisonOnUpdateRequiresSecondaryTimeRange checks
// that switching a stored report to layout "cumulative_comparison" without a
// secondary_time_range is rejected at plan time. On update an omitted
// secondary_time_range plans unknown, not as the stored (absent) value, so a
// validator that skips unknown values never reports it.
func TestAccReport_CumulativeComparisonOnUpdateRequiresSecondaryTimeRange(t *testing.T) {
	n := acctest.RandInt()
	config := func(layout string) string {
		return fmt.Sprintf(`
resource "doit_report" "test" {
  name = "test-cumulative-secondary-%d"
  config = {
    layout        = %q
    aggregation   = "total"
    currency      = "USD"
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    dimensions = [
      { id = "year", type = "datetime" },
      { id = "month", type = "datetime" },
      { id = "day", type = "datetime" },
    ]
    metrics = [{ type = "basic", value = "cost" }]
  }
}
`, n, layout)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config("table")},
			{
				Config:      config("cumulative_comparison"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Missing Secondary Time Range`),
			},
		},
	})
}

// TestAccReport_OmittedDataSourceKeepsStoredValueOnUpdate checks that dropping
// data_source from the config of a report stored with a non-default data source
// (billing; the CI tenant default is billing-datahub) does not change it. The omitted attribute plans unknown on update and the
// request omits dataSource, which the API may reset to the tenant default.
func TestAccReport_OmittedDataSourceKeepsStoredValueOnUpdate(t *testing.T) {
	n := acctest.RandInt()
	config := func(dataSource, currency string) string {
		return fmt.Sprintf(`
# Control: created without data_source, so it shows the tenant default
# (billing-datahub). test stores billing, which differs, so the check on test
# below cannot pass by accident.
resource "doit_report" "control" {
  name = "test-omitted-data-source-control-%[1]d"
  config = {
    currency      = "USD"
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    metrics = [{ type = "basic", value = "cost" }]
  }
}

resource "doit_report" "test" {
  name = "test-omitted-data-source-keeps-%[1]d"
  config = {
    %[2]s
    currency      = %[3]q
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 7
      unit            = "day"
      include_current = true
    }
    metrics = [{ type = "basic", value = "cost" }]
  }
}
`, n, dataSource, currency)
	}
	dataSourcePath := tfjsonpath.New("config").AtMapKey("data_source")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config(`data_source = "billing"`, "USD"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.test", dataSourcePath, knownvalue.StringExact("billing")),
					statecheck.ExpectKnownValue("doit_report.control", dataSourcePath, knownvalue.StringExact("billing-datahub")),
				},
			},
			{
				Config: config("", "EUR"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.test", dataSourcePath, knownvalue.StringExact("billing")),
				},
			},
		},
	})
}
