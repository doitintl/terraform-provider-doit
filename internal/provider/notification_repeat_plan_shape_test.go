package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// testAccCheckNotificationEtagCurrent compares the stored etag with the one the
// API returns, so a stale etag left by an update fails the step.
func testAccCheckNotificationEtagCurrent(t *testing.T, addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not found in state", addr)
		}
		client := getAPIClient(t)
		var etag string
		switch rs.Type {
		case "doit_user_notification":
			resp, err := client.GetUserNotificationWithResponse(t.Context(), rs.Primary.Attributes["user_id"], rs.Primary.ID)
			if err != nil || resp.JSON200 == nil {
				return fmt.Errorf("reading %s: %v", addr, err)
			}
			etag, _ = resp.JSON200.Etag.Get()
		default:
			resp, err := client.GetCustomerNotificationWithResponse(t.Context(), rs.Primary.ID)
			if err != nil || resp.JSON200 == nil {
				return fmt.Errorf("reading %s: %v", addr, err)
			}
			etag, _ = resp.JSON200.Etag.Get()
		}
		if got := rs.Primary.Attributes["etag"]; etag == "" || got != etag {
			return fmt.Errorf("%s: etag in state %q, API returns %q", addr, got, etag)
		}
		return nil
	}
}

// An update whose only change comes from a clearing plan modifier (Terraform
// proposed the stored value) must still plan etag and update_time unknown, or
// state keeps the old etag and the next update without refresh fails with 412.
func testAccClearingOnlyUpdateStep(t *testing.T, addr, cfg string) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				plancheck.ExpectUnknownValue(addr, tfjsonpath.New("etag")),
				plancheck.ExpectUnknownValue(addr, tfjsonpath.New("update_time")),
			},
		},
		Check: testAccCheckNotificationEtagCurrent(t, addr),
	}
}

func TestAccUserNotification_UnsubscribeRefreshesEtag(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-etag")
	both := testAccUserNotificationSettingsConfig(rName, rName, `
    credit_utilization = {}
    cloud_incidents    = {}
`)
	one := testAccUserNotificationSettingsConfig(rName, rName, `
    credit_utilization = {}
`)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: both},
			testAccClearingOnlyUpdateStep(t, testAccUserNotificationAddr, one),
			testAccUserNotificationDriftCheck(one),
		},
	})
}

func TestAccCustomerNotification_ClearControlRefreshesEtag(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-etag")
	const base = `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	withWindow := testAccCustomerNotificationConfig(rName, rName, base+`
      repeat = { when_cost_grows = { percent_increase = 10, within_hours = 6 } }
`)
	withoutWindow := testAccCustomerNotificationConfig(rName, rName, base+`
      repeat = { when_cost_grows = { percent_increase = 10 } }
`)
	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: withWindow},
			testAccClearingOnlyUpdateStep(t, testAccCustomerNotificationAddr, withoutWindow),
			testAccCustomerNotificationDriftCheck(withoutWindow),
		},
	})
}
