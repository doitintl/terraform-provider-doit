package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccReport_LayoutUpdates verifies that the API persists layout changes, the
// report data source reads them back, and each refreshed configuration has no drift.
func TestAccReport_LayoutUpdates(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-report-layout")
	var steps []resource.TestStep
	for i, layout := range []string{"sankey_chart", "column_and_line_chart", "trend_board"} {
		config := testAccReportLayoutConfig(name, layout)
		action := plancheck.ResourceActionUpdate
		if i == 0 {
			action = plancheck.ResourceActionCreate
		}
		checks := []statecheck.StateCheck{
			statecheck.ExpectKnownValue("doit_report.test", tfjsonpath.New("config").AtMapKey("layout"), knownvalue.StringExact(layout)),
			statecheck.ExpectKnownValue("data.doit_report.test", tfjsonpath.New("config").AtMapKey("layout"), knownvalue.StringExact(layout)),
		}
		steps = append(steps,
			resource.TestStep{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_report.test", action),
					},
				},
				ConfigStateChecks: checks,
			},
			resource.TestStep{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: checks,
			},
		)
	}
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    steps,
	})
}

func testAccReportLayoutConfig(name, layout string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
  name = %q
  config = {
    metric = { type = "basic", value = "cost" }
    aggregation = "total"
    time_interval = "month"
    data_source = "billing"
    display_values = "actuals_only"
    currency = "USD"
    dimensions = [{ id = "month", type = "datetime" }]
    layout = %q
  }
}

data "doit_report" "test" {
  id = doit_report.test.id
}
`, name, layout)
}
