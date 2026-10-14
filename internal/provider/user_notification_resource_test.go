package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testAccUserNotificationAddr = "doit_user_notification.test"

func testAccUserNotificationPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheckFunc(t)()
	if os.Getenv("TEST_USER_NOTIFICATION_USER_ID") == "" {
		t.Skip("TEST_USER_NOTIFICATION_USER_ID must be set for this test")
	}
	if os.Getenv("TEST_USER_NOTIFICATION_ALLOCATION_ID") == "" {
		t.Skip("TEST_USER_NOTIFICATION_ALLOCATION_ID must be set for this test")
	}
}

func TestAccUserNotification_Lifecycle(t *testing.T) {
	userID := os.Getenv("TEST_USER_NOTIFICATION_USER_ID")
	allocID := os.Getenv("TEST_USER_NOTIFICATION_ALLOCATION_ID")
	rName := acctest.RandomWithPrefix("tf-acc-un")

	createCfg := testAccUserNotificationConfig(userID, rName, allocID)
	updateCfg := testAccUserNotificationConfig(userID, rName+"-upd", allocID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 func() { testAccUserNotificationPreCheck(t) },
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: createCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("user_id"), knownvalue.StringExact(userID)),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						testAccUserNotificationAddr,
						tfjsonpath.New("settings").AtMapKey("cost_anomalies").AtMapKey("min_excess_cost").AtMapKey("amount"),
						knownvalue.StringExact("100.00"),
					),
				},
			},
			{
				Config: createCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: updateCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-upd")),
				},
			},
			{
				Config: updateCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            testAccUserNotificationAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts", "etag"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[testAccUserNotificationAddr]
					if !ok {
						return "", fmt.Errorf("resource %s not found in state", testAccUserNotificationAddr)
					}
					return rs.Primary.Attributes["user_id"] + "/" + rs.Primary.ID, nil
				},
			},
			{
				Config: updateCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccUserNotification_ForEach(t *testing.T) {
	userID := os.Getenv("TEST_USER_NOTIFICATION_USER_ID")
	userID2 := os.Getenv("TEST_USER_NOTIFICATION_USER_ID_2")
	allocID := os.Getenv("TEST_USER_NOTIFICATION_ALLOCATION_ID")
	if userID2 == "" {
		t.Skip("TEST_USER_NOTIFICATION_USER_ID_2 must be set for this test")
	}
	rName := acctest.RandomWithPrefix("tf-acc-un-fe")

	cfg := testAccUserNotificationForEachConfig(userID, userID2, rName, allocID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 func() { testAccUserNotificationPreCheck(t) },
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccUserNotificationConfig(userID, name, allocationID string) string {
	return fmt.Sprintf(`
resource "doit_user_notification" "test" {
  user_id = %q
  name    = %q

  settings = {
    cost_anomalies = {
      min_severity = "warning"
      min_excess_cost = {
        amount   = "100.00"
        currency = "USD"
      }
      allocations = {
        mode = "include"
        ids  = [%q]
      }
      repeat = {
        every_hours = 24
      }
    }
  }
}
`, userID, name, allocationID)
}

func testAccUserNotificationForEachConfig(userID, userID2, name, allocationID string) string {
	return fmt.Sprintf(`
locals {
  users = {
    a = %q
    b = %q
  }
}

resource "doit_user_notification" "test" {
  for_each = local.users

  user_id = each.value
  name    = "%s-${each.key}"

  settings = {
    cost_anomalies = {
      min_severity = "warning"
      min_excess_cost = {
        amount   = "100.00"
        currency = "USD"
      }
      allocations = {
        mode = "include"
        ids  = [%q]
      }
      repeat = {
        every_hours = 24
      }
    }
  }
}
`, userID, userID2, name, allocationID)
}
