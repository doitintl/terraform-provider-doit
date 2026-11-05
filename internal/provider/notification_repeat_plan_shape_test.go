package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccRepeatPlanShapeSteps pins the plan of omitted repeat controls of a
// cost-anomaly notification. Omitted objects (when_cost_grows, min_increase)
// plan null, whether or not they are stored. Omitted scalars plan unknown when
// nothing is stored (the API fills them) and null when a stored value is
// cleared.
func testAccRepeatPlanShapeSteps(addr string, path func(keys ...string) tfjsonpath.Path, config func(costAnomalies string) string) []resource.TestStep {
	const base = `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	percentOnly := config(base + `
      repeat = {
        when_cost_grows = { percent_increase = 10 }
      }
`)
	full := config(base + `
      repeat = {
        every_hours = 12
        when_cost_grows = {
          match            = "and"
          percent_increase = 10
          min_increase     = { amount = "150", currency = "USD" }
          within_hours     = 6
        }
      }
`)
	withoutMinIncrease := config(base + `
      repeat = {
        every_hours = 12
        when_cost_grows = {
          match            = "or"
          percent_increase = 10
        }
      }
`)
	everyHoursOnly := config(base + `
      repeat = { every_hours = 12 }
`)

	update := func(cfg string, checks ...plancheck.PlanCheck) resource.TestStep {
		return resource.TestStep{
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: append([]plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)}, checks...),
			},
		}
	}
	drift := func(cfg string) resource.TestStep {
		return resource.TestStep{
			Config:           cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
		}
	}

	return []resource.TestStep{
		// Create with only percent_increase: nothing is stored yet.
		{
			Config: percentOnly,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue(addr, path("repeat", "every_hours")),
					plancheck.ExpectUnknownValue(addr, path("repeat", "when_cost_grows", "match")),
					plancheck.ExpectUnknownValue(addr, path("repeat", "when_cost_grows", "within_hours")),
					plancheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows", "min_increase"), knownvalue.Null()),
					plancheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows", "percent_increase"), knownvalue.Int64Exact(10)),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(addr, path("repeat", "every_hours"), knownvalue.Null()),
				statecheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows", "match"), knownvalue.Null()),
				statecheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows", "within_hours"), knownvalue.Null()),
			},
		},
		drift(percentOnly),
		update(full),
		// Remove the stored min_increase and within_hours.
		update(withoutMinIncrease,
			plancheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows", "min_increase"), knownvalue.Null()),
			plancheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows", "within_hours"), knownvalue.Null()),
			plancheck.ExpectKnownValue(addr, path("repeat", "every_hours"), knownvalue.Int64Exact(12)),
		),
		drift(withoutMinIncrease),
		// Remove the stored when_cost_grows.
		update(everyHoursOnly,
			plancheck.ExpectKnownValue(addr, path("repeat", "when_cost_grows"), knownvalue.Null()),
		),
		drift(everyHoursOnly),
	}
}

func TestAccUserNotification_RepeatPlanShape(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-rep-plan")
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: testAccRepeatPlanShapeSteps(testAccUserNotificationAddr, testAccUserNotificationPath, func(costAnomalies string) string {
			return testAccUserNotificationConfig(rName, rName, costAnomalies)
		}),
	})
}

func TestAccCustomerNotification_RepeatPlanShape(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-rep-plan")
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: testAccRepeatPlanShapeSteps(testAccCustomerNotificationAddr, testAccCustomerNotificationPath, func(costAnomalies string) string {
			return testAccCustomerNotificationConfig(rName, rName, costAnomalies)
		}),
	})
}
