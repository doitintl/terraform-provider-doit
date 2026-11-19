package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The notifications are delivered to TEST_USER and only live for the
// duration of a test. The CI tenant has no Microsoft Teams entitlement, so a
// Teams destination fails at the API with any type; the Teams rule of
// expert_insights and data_integration_sync_failure is covered at plan time.

// testAccCustomerNotificationSettingsPath returns a fresh path under settings.
func testAccCustomerNotificationSettingsPath(keys ...string) tfjsonpath.Path {
	p := tfjsonpath.New("settings")
	for _, k := range keys {
		p = p.AtMapKey(k)
	}
	return p
}

// testAccCustomerNotificationCostAnomalies is a cost_anomalies block with a
// threshold no real anomaly reaches.
const testAccCustomerNotificationCostAnomalies = `
    cost_anomalies = {
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
    }
`

// testAccCustomerNotificationSettingsConfig is testAccCustomerNotificationConfig
// with the whole settings body spliced in, for notifications with other types
// than cost anomalies.
func testAccCustomerNotificationSettingsConfig(rName, name, settings string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "a" {
  name        = "%[1]s-a"
  description = "customer notification test allocation"
  rule = {
    formula    = "A"
    components = [{ key = "country", mode = "is", type = "fixed", values = ["JP"] }]
  }
}

resource "doit_customer_notification" "this" {
  name         = %[2]q
  destinations = { emails = [%[3]q], slack_channels = [], teams_channels = [] }
  settings = {
%[4]s
  }
}
`, rName, name, testUser(), settings)
}

// Types of every shape are subscribed without cost anomalies, then switched in
// one update. The switch replaces the insight filters pillars and severities
// with providers: the API merges a sent type into the stored one, so pillars
// and severities are only cleared because the update sends them as [].
// Omitted filter lists are [] in state and stay [] when another attribute
// changes.
func TestAccCustomerNotification_SettingsTypes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-types")

	created := testAccCustomerNotificationSettingsConfig(rName, rName, `
    credit_utilization      = {}
    cloud_incidents         = {}
    new_invoices            = {}
    aws_billing_data_import = {}
    insights = {
      pillars    = ["security", "cost"]
      severities = ["critical"]
    }
`)
	switched := testAccCustomerNotificationSettingsConfig(rName, rName, `
    new_invoices            = {}
    aws_billing_data_import = {}
    daily_digest            = { allocation_ids = [doit_allocation.a.id] }
    insights                = { providers = ["aws"] }
`+testAccCustomerNotificationCostAnomalies)
	renamed := testAccCustomerNotificationSettingsConfig(rName, rName+"-renamed", `
    new_invoices            = {}
    aws_billing_data_import = {}
    daily_digest            = { allocation_ids = [doit_allocation.a.id] }
    insights                = { providers = ["aws"] }
`+testAccCustomerNotificationCostAnomalies)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: created,
				// An omitted type plans null on create, not unknown.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("daily_digest"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("credit_utilization"), knownvalue.ObjectExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cloud_incidents"), knownvalue.ObjectExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("aws_billing_data_import", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "pillars"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("security"), knownvalue.StringExact("cost")})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "providers"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("daily_digest"), knownvalue.Null()),
				},
			},
			testAccCustomerNotificationDriftCheck(created),
			// Unsubscribe two types, clear two filter lists, set another, and
			// subscribe to two new types (one of them cost anomalies) in one update.
			{
				Config: switched,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("credit_utilization"), knownvalue.Null()),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "pillars"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "severities"), testAccEmptyList),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("credit_utilization"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cloud_incidents"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "pillars"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "severities"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "providers"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("aws")})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("daily_digest", "allocation_ids"),
						knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cost_anomalies", "min_severity"), knownvalue.StringExact("warning")),
				},
			},
			testAccCustomerNotificationDriftCheck(switched),
			// A change outside settings plans the omitted filter lists as []
			// and the omitted types as null, not unknown.
			{
				Config: renamed,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "pillars"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("insights", "severities"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("credit_utilization"), knownvalue.Null()),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("weekly_digest"), knownvalue.Null()),
					},
				},
			},
			testAccCustomerNotificationDriftCheck(renamed),
		},
	})
}

// billing_profile_ids is set on two types. Removing it from one config clears
// it: the plan proposes [], the update sends [] (the API merges an omitted list
// into the stored one), and the API stores every profile (absent), which reads
// back as []. The other is then cleared in the same step as a rename.
func TestAccCustomerNotification_BillingProfileIDs(t *testing.T) {
	billingProfileID := os.Getenv("TEST_BILLING_PROFILE_ID")
	if billingProfileID == "" {
		t.Skip("TEST_BILLING_PROFILE_ID must be set")
	}
	rName := acctest.RandomWithPrefix("tf-acc-cn-billing")

	set := testAccCustomerNotificationSettingsConfig(rName, rName, fmt.Sprintf(`
    new_invoices            = { billing_profile_ids = [%[1]q] }
    aws_billing_data_import = { billing_profile_ids = [%[1]q] }
`, billingProfileID))
	cleared := testAccCustomerNotificationSettingsConfig(rName, rName, fmt.Sprintf(`
    new_invoices            = {}
    aws_billing_data_import = { billing_profile_ids = [%[1]q] }
`, billingProfileID))
	clearedWithRename := testAccCustomerNotificationSettingsConfig(rName, rName+"-renamed", `
    new_invoices            = {}
    aws_billing_data_import = {}
`)
	profile := knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(billingProfileID)})

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: set,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("new_invoices", "billing_profile_ids"), profile),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("aws_billing_data_import", "billing_profile_ids"), profile),
				},
			},
			testAccCustomerNotificationDriftCheck(set),
			{
				Config: cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("aws_billing_data_import", "billing_profile_ids"), profile),
				},
			},
			testAccCustomerNotificationDriftCheck(cleared),
			{
				Config: clearedWithRename,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("aws_billing_data_import", "billing_profile_ids"), testAccEmptyList),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("aws_billing_data_import", "billing_profile_ids"), testAccEmptyList),
				},
			},
			testAccCustomerNotificationDriftCheck(clearedWithRename),
		},
	})
}

// Removing cost_anomalies unsubscribes it. Nothing else changes, so the update
// comes only from the clearing modifier and must still plan etag unknown.
// Adding it back subscribes it again.
func TestAccCustomerNotification_RemoveCostAnomalies(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-rm-ca")
	both := testAccCustomerNotificationSettingsConfig(rName, rName, `
    credit_utilization = {}
`+testAccCustomerNotificationCostAnomalies)
	withoutCostAnomalies := testAccCustomerNotificationSettingsConfig(rName, rName, `
    credit_utilization = {}
`)

	removeStep := testAccClearingOnlyUpdateStep(t, testAccCustomerNotificationAddr, withoutCostAnomalies)
	removeStep.ConfigStateChecks = []statecheck.StateCheck{
		statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
	}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: both},
			removeStep,
			testAccCustomerNotificationDriftCheck(withoutCostAnomalies),
			{
				Config: both,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationSettingsPath("cost_anomalies", "min_severity"), knownvalue.StringExact("warning")),
				},
			},
			testAccCustomerNotificationDriftCheck(both),
		},
	})
}

// Import maps every subscribed type, and configuration generated for it
// ({} objects and [] lists included) is valid and plans no change. The amount
// uses the API's spelling, which import reads back (see
// TestAccCustomerNotification_ImportAmountSpelling).
func TestAccCustomerNotification_SettingsTypesImport(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-types-imp")

	cfg := testAccCustomerNotificationSettingsConfig(rName, rName, `
    new_invoices            = { billing_profile_ids = [] }
    new_proforma_invoices   = {}
    payment_reminders       = {}
    credit_utilization      = {}
    quota_utilization       = {}
    cloud_incidents         = {}
    new_sku_usage           = {}
    daily_digest            = { allocation_ids = [doit_allocation.a.id] }
    weekly_digest           = { allocation_ids = [doit_allocation.a.id] }
    monthly_digest          = {}
    expert_insights         = {}
    insights = {
      pillars    = ["cost"]
      severities = ["high", "critical"]
      providers  = ["gcp"]
    }
    savings_plan_approval         = {}
    savings_plan_purchased        = {}
    cud_approval                  = {}
    cud_purchased                 = {}
    data_integration_sync_failure = {}
    aws_billing_data_import       = {}
    cost_anomalies = {
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000.00", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
    }
`)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			testAccCustomerNotificationDriftCheck(cfg),
			{
				ResourceName:            testAccCustomerNotificationAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				ResourceName:    testAccCustomerNotificationAddr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
			testAccCustomerNotificationDriftCheck(cfg),
		},
	})
}

// A notification with a single type without fields: configuration generated
// on import holds that {} and every other type as null.
func TestAccCustomerNotification_MinimalSettingsImport(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-min-imp")
	cfg := testAccCustomerNotificationSettingsConfig(rName, rName, `
    monthly_digest = {}
`)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				ResourceName:            testAccCustomerNotificationAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				ResourceName:    testAccCustomerNotificationAddr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
		},
	})
}

func TestAccCustomerNotification_SettingsValidation(t *testing.T) {
	const teams = `{ emails = [], slack_channels = [], teams_channels = ["19:abc@thread.tacv2"] }`
	const email = `{ emails = ["finops@example.com"], slack_channels = [], teams_channels = [] }`
	cases := map[string]struct {
		destinations string
		settings     string
		err          string
	}{
		"no_type": {
			destinations: email,
			settings:     ``,
			err:          `No Notification Type`,
		},
		"teams_expert_insights": {
			destinations: teams,
			settings:     `expert_insights = {}`,
			err:          `expert_insights.*cannot be delivered to\s+Microsoft\s+Teams`,
		},
		"teams_data_integration_sync_failure": {
			destinations: teams,
			settings:     `data_integration_sync_failure = {}`,
			err:          `data_integration_sync_failure.*cannot be\s+delivered to\s+Microsoft\s+Teams`,
		},
		"unknown_pillar": {
			destinations: email,
			settings:     `insights = { pillars = ["costs"] }`,
			err:          `value must be\s+one\s+of`,
		},
		"duplicate_severity": {
			destinations: email,
			settings:     `insights = { severities = ["high", "high"] }`,
			err:          `duplicate\s+values`,
		},
		"duplicate_billing_profile": {
			destinations: email,
			settings:     `aws_billing_data_import = { billing_profile_ids = ["ent_1", "ent_1"] }`,
			err:          `duplicate\s+values`,
		},
		"empty_digest": {
			destinations: email,
			settings:     `weekly_digest = { allocation_ids = [] }`,
			err:          `at\s+least\s+1`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: fmt.Sprintf(`
resource "doit_customer_notification" "this" {
  name         = "validation"
  destinations = %s
  settings = {
    %s
  }
}
`, tc.destinations, tc.settings),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(tc.err),
					},
				},
			})
		})
	}
}

func TestAccCustomerNotificationDataSource_SettingsTypes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-ds-types")
	config := testAccCustomerNotificationSettingsConfig(rName, rName, `
    credit_utilization      = {}
    aws_billing_data_import = {}
    daily_digest            = { allocation_ids = [doit_allocation.a.id] }
    insights                = { pillars = ["security"] }
`) + `
data "doit_customer_notification" "test" {
  id = doit_customer_notification.this.id
}

data "doit_customer_notifications" "test" {
  depends_on = [doit_customer_notification.this]
}
`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationDataSourceAddr, "settings.daily_digest.allocation_ids.0", "doit_allocation.a", "id"),
					testAccCheckCustomerNotificationsItem(rName, map[string]string{
						"settings.credit_utilization.%":                          "0",
						"settings.aws_billing_data_import.billing_profile_ids.#": "0",
						"settings.daily_digest.allocation_ids.#":                 "1",
						"settings.insights.pillars.0":                            "security",
						"settings.insights.providers.#":                          "0",
					}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, testAccCustomerNotificationSettingsPath("credit_utilization"), knownvalue.ObjectExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, testAccCustomerNotificationSettingsPath("cloud_incidents"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, testAccCustomerNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, testAccCustomerNotificationSettingsPath("aws_billing_data_import", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, testAccCustomerNotificationSettingsPath("insights", "severities"), testAccEmptyList),
				},
			},
			testAccCustomerNotificationDriftCheck(config),
		},
	})
}
