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

// The CI user holds every permission the notification types need, so every
// type can be subscribed. Digests and insights would email the user, but the
// notifications only live for the duration of a test.

// testAccUserNotificationSettingsPath returns a fresh path under settings.
func testAccUserNotificationSettingsPath(keys ...string) tfjsonpath.Path {
	p := tfjsonpath.New("settings")
	for _, k := range keys {
		p = p.AtMapKey(k)
	}
	return p
}

// testAccUserNotificationSettingsConfig is testAccUserNotificationConfig with
// the whole settings body spliced in, for notifications without cost anomalies.
func testAccUserNotificationSettingsConfig(rName, name, settings string) string {
	return fmt.Sprintf(`
data "doit_users" "test" {
  email = %[1]q
}

resource "doit_allocation" "a" {
  name        = "%[2]s-a"
  description = "user notification test allocation"
  rule = {
    formula    = "A"
    components = [{ key = "country", mode = "is", type = "fixed", values = ["JP"] }]
  }
}

resource "doit_user_notification" "this" {
  user_id = data.doit_users.test.users[0].id
  name    = %[3]q
  settings = {
%[4]s
  }
}
`, testUser(), rName, name, settings)
}

var testAccEmptyList = knownvalue.ListExact([]knownvalue.Check{})

// Types are subscribed by setting them, unsubscribed by removing them, and
// switched in one update. Omitted filter lists are [] in state and stay []
// when another attribute changes.
func TestAccUserNotification_SettingsTypes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-types")

	created := testAccUserNotificationSettingsConfig(rName, rName, `
    credit_utilization = {}
    cloud_incidents    = {}
    new_invoices       = {}
    insights = {
      pillars    = ["security", "cost"]
      severities = ["critical"]
    }
`)
	switched := testAccUserNotificationSettingsConfig(rName, rName, `
    new_invoices = {}
    daily_digest = { allocation_ids = [doit_allocation.a.id] }
    insights     = { providers = ["aws"] }
    cost_anomalies = {
`+testAccUserNotificationMinimalSettings+`
    }
`)
	renamed := testAccUserNotificationSettingsConfig(rName, rName+"-renamed", `
    new_invoices = {}
    daily_digest = { allocation_ids = [doit_allocation.a.id] }
    insights     = { providers = ["aws"] }
    cost_anomalies = {
`+testAccUserNotificationMinimalSettings+`
    }
`)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: created,
				// An omitted type plans null on create, not unknown.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("daily_digest"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("credit_utilization"), knownvalue.ObjectExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("cloud_incidents"), knownvalue.ObjectExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "pillars"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("security"), knownvalue.StringExact("cost")})),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "providers"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("daily_digest"), knownvalue.Null()),
				},
			},
			testAccUserNotificationDriftCheck(created),
			// Unsubscribe two types, clear a filter list, set another, and
			// subscribe to two new types in one update.
			{
				Config: switched,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("credit_utilization"), knownvalue.Null()),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "pillars"), testAccEmptyList),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("credit_utilization"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("cloud_incidents"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "pillars"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "severities"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "providers"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("aws")})),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("daily_digest", "allocation_ids"),
						knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("cost_anomalies", "min_severity"), knownvalue.StringExact("warning")),
				},
			},
			testAccUserNotificationDriftCheck(switched),
			// A change outside settings plans the omitted filter lists as []
			// and the omitted types as null, not unknown.
			{
				Config: renamed,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "pillars"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("insights", "severities"), testAccEmptyList),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("credit_utilization"), knownvalue.Null()),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("weekly_digest"), knownvalue.Null()),
					},
				},
			},
			testAccUserNotificationDriftCheck(renamed),
		},
	})
}

// billing_profile_ids is set, then cleared by removing it from config: the
// plan proposes [], the update sends [], and the API stores every profile
// (absent), which reads back as [].
func TestAccUserNotification_BillingProfileIDs(t *testing.T) {
	billingProfileID := os.Getenv("TEST_BILLING_PROFILE_ID")
	if billingProfileID == "" {
		t.Skip("TEST_BILLING_PROFILE_ID must be set")
	}
	rName := acctest.RandomWithPrefix("tf-acc-un-billing")

	set := testAccUserNotificationSettingsConfig(rName, rName, fmt.Sprintf(`
    new_invoices      = { billing_profile_ids = [%[1]q] }
    payment_reminders = { billing_profile_ids = [%[1]q] }
`, billingProfileID))
	cleared := testAccUserNotificationSettingsConfig(rName, rName, fmt.Sprintf(`
    new_invoices      = {}
    payment_reminders = { billing_profile_ids = [%[1]q] }
`, billingProfileID))
	profile := knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(billingProfileID)})

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: set,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("new_invoices", "billing_profile_ids"), profile),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("payment_reminders", "billing_profile_ids"), profile),
				},
			},
			testAccUserNotificationDriftCheck(set),
			{
				Config: cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationSettingsPath("payment_reminders", "billing_profile_ids"), profile),
				},
			},
			testAccUserNotificationDriftCheck(cleared),
		},
	})
}

// Import maps every subscribed type, and configuration generated for it
// ({} objects and [] lists included) is valid and plans no change.
func TestAccUserNotification_SettingsTypesImport(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-types-imp")

	cfg := testAccUserNotificationSettingsConfig(rName, rName, `
    new_invoices          = { billing_profile_ids = [] }
    new_proforma_invoices = {}
    payment_reminders     = {}
    credit_utilization    = {}
    quota_utilization     = {}
    cloud_incidents       = {}
    new_sku_usage         = {}
    daily_digest          = { allocation_ids = [doit_allocation.a.id] }
    weekly_digest         = { allocation_ids = [doit_allocation.a.id] }
    monthly_digest        = {}
    expert_insights       = {}
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
`)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			testAccUserNotificationDriftCheck(cfg),
			{
				ResourceName:            testAccUserNotificationAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
				ImportStateIdFunc:       testAccUserNotificationImportID,
			},
			{
				ResourceName:      testAccUserNotificationAddr,
				ImportStateIdFunc: testAccUserNotificationImportID,
				ImportState:       true,
				ImportStateKind:   resource.ImportBlockWithID,
				GenerateConfig:    true,
			},
			testAccUserNotificationDriftCheck(cfg),
		},
	})
}

func TestAccUserNotification_SettingsValidation(t *testing.T) {
	cases := map[string]struct {
		settings string
		err      string
	}{
		"no_type": {
			settings: ``,
			err:      `No Notification Type`,
		},
		"unknown_pillar": {
			settings: `insights = { pillars = ["costs"] }`,
			err:      `value must be\s+one\s+of`,
		},
		"duplicate_severity": {
			settings: `insights = { severities = ["high", "high"] }`,
			err:      `duplicate\s+values`,
		},
		"empty_provider": {
			settings: `insights = { providers = [""] }`,
			err:      `length\s+must\s+be\s+at\s+least\s+1`,
		},
		"duplicate_billing_profile": {
			settings: `new_invoices = { billing_profile_ids = ["ent_1", "ent_1"] }`,
			err:      `duplicate\s+values`,
		},
		"empty_digest": {
			settings: `daily_digest = { allocation_ids = [] }`,
			err:      `at\s+least\s+1`,
		},
		"duplicate_digest_allocation": {
			settings: `weekly_digest = { allocation_ids = ["alloc", "alloc"] }`,
			err:      `duplicate\s+values`,
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
resource "doit_user_notification" "this" {
  user_id = "user-id"
  name    = "validation"
  settings = {
    %s
  }
}
`, tc.settings),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(tc.err),
					},
				},
			})
		})
	}
}

func TestAccUserNotificationDataSource_SettingsTypes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-ds-types")
	config := testAccUserNotificationSettingsConfig(rName, rName, `
    credit_utilization = {}
    new_invoices       = {}
    daily_digest       = { allocation_ids = [doit_allocation.a.id] }
    insights           = { pillars = ["security"] }
`) + `
data "doit_user_notification" "test" {
  user_id = doit_user_notification.this.user_id
  id      = doit_user_notification.this.id
}

data "doit_user_notifications" "test" {
  user_id    = doit_user_notification.this.user_id
  depends_on = [doit_user_notification.this]
}
`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "settings.daily_digest.allocation_ids.0", "doit_allocation.a", "id"),
					testAccCheckUserNotificationsItem(rName, map[string]string{
						"settings.credit_utilization.%":               "0",
						"settings.new_invoices.billing_profile_ids.#": "0",
						"settings.daily_digest.allocation_ids.#":      "1",
						"settings.insights.pillars.0":                 "security",
						"settings.insights.providers.#":               "0",
					}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationDataSourceAddr, testAccUserNotificationSettingsPath("credit_utilization"), knownvalue.ObjectExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccUserNotificationDataSourceAddr, testAccUserNotificationSettingsPath("cloud_incidents"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccUserNotificationDataSourceAddr, testAccUserNotificationSettingsPath("cost_anomalies"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccUserNotificationDataSourceAddr, testAccUserNotificationSettingsPath("new_invoices", "billing_profile_ids"), testAccEmptyList),
					statecheck.ExpectKnownValue(testAccUserNotificationDataSourceAddr, testAccUserNotificationSettingsPath("insights", "severities"), testAccEmptyList),
				},
			},
			testAccUserNotificationDriftCheck(config),
		},
	})
}
