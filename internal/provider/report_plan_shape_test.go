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
