package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testAccCustomerNotificationAddr = "doit_customer_notification.this"

// The notifications are delivered to TEST_USER. Every test uses a very high
// min_excess_cost so nobody is emailed about real anomalies.

// testAccCustomerNotificationPath returns a fresh path under settings.cost_anomalies.
// Paths are built per call: deriving several paths from one shared prefix can
// alias its backing array and overwrite earlier paths.
func testAccCustomerNotificationPath(keys ...string) tfjsonpath.Path {
	p := tfjsonpath.New("settings").AtMapKey("cost_anomalies")
	for _, k := range keys {
		p = p.AtMapKey(k)
	}
	return p
}

func TestAccCustomerNotification_Lifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn")

	// Amounts are written without decimals on purpose: the API answers with
	// "1000000.00", and state must keep the configured spelling.
	all := testAccCustomerNotificationConfig(rName, rName, `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`)
	// b before a: the API keeps the configured order.
	include := testAccCustomerNotificationConfig(rName, rName+"-renamed", `
      min_severity    = "critical"
      min_excess_cost = { amount = "2000000.00", currency = "USD" }
      allocations     = { mode = "include", ids = [doit_allocation.b.id, doit_allocation.a.id] }
`)
	exclude := testAccCustomerNotificationConfig(rName, rName+"-renamed", `
      min_severity    = "critical"
      min_excess_cost = { amount = "2000000.00", currency = "USD" }
      allocations     = { mode = "exclude", ids = [doit_allocation.a.id] }
`)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with mode "all" and no repeat
			{
				Config: all,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("create_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("update_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("destinations"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"emails":         knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(testUser())}),
						"slack_channels": knownvalue.ListExact([]knownvalue.Check{}),
						"teams_channels": knownvalue.ListExact([]knownvalue.Check{}),
					})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("1000000")),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("allocations", "ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat"), knownvalue.Null()),
				},
			},
			// Step 2: Drift check
			testAccCustomerNotificationDriftCheck(all),
			// Step 3: Update name, severity, amount and allocations in place
			{
				Config: include,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-renamed")),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("min_severity"), knownvalue.StringExact("critical")),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("2000000.00")),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("allocations", "mode"), knownvalue.StringExact("include")),
					statecheck.CompareValuePairs(
						testAccCustomerNotificationAddr, testAccCustomerNotificationPath("allocations", "ids").AtSliceIndex(0),
						"doit_allocation.b", tfjsonpath.New("id"), compare.ValuesSame()),
					statecheck.CompareValuePairs(
						testAccCustomerNotificationAddr, testAccCustomerNotificationPath("allocations", "ids").AtSliceIndex(1),
						"doit_allocation.a", tfjsonpath.New("id"), compare.ValuesSame()),
				},
			},
			// Step 4: Drift check (order of ids is kept)
			testAccCustomerNotificationDriftCheck(include),
			// Step 5: Switch to exclude with a shorter list
			{
				Config: exclude,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("allocations", "ids"), knownvalue.ListSizeExact(1)),
				},
			},
			// Step 6: Drift check
			testAccCustomerNotificationDriftCheck(exclude),
			// Step 7: Back to mode "all" with an empty list
			{
				Config: all,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("allocations", "ids"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Step 8: Drift check
			testAccCustomerNotificationDriftCheck(all),
		},
	})
}

// TestAccCustomerNotification_Repeat covers clearing repeat in steps: single
// leaves, the when_cost_grows branch, the whole block, and setting it again.
func TestAccCustomerNotification_Repeat(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-rep")

	base := `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	full := testAccCustomerNotificationConfig(rName, rName, base+`
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
	percentOnly := testAccCustomerNotificationConfig(rName, rName, base+`
      repeat = {
        every_hours = 12
        when_cost_grows = {
          percent_increase = 10
        }
      }
`)
	everyHoursOnly := testAccCustomerNotificationConfig(rName, rName, base+`
      repeat = {
        every_hours = 12
      }
`)
	growthOnly := testAccCustomerNotificationConfig(rName, rName, base+`
      repeat = {
        when_cost_grows = {
          min_increase = { amount = "250.00", currency = "USD" }
        }
      }
`)
	noRepeat := testAccCustomerNotificationConfig(rName, rName, base)

	updateStep := func(cfg string, checks ...statecheck.StateCheck) resource.TestStep {
		return resource.TestStep{
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: checks,
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with every resend control set
			{
				Config: full,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "every_hours"), knownvalue.Int64Exact(12)),
					statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "when_cost_grows"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"match":            knownvalue.StringExact("and"),
						"percent_increase": knownvalue.Int64Exact(10),
						"min_increase": knownvalue.ObjectExact(map[string]knownvalue.Check{
							"amount":   knownvalue.StringExact("150"),
							"currency": knownvalue.StringExact("USD"),
						}),
						"within_hours": knownvalue.Int64Exact(6),
					})),
				},
			},
			testAccCustomerNotificationDriftCheck(full),
			// Step 3: Clear single leaves inside when_cost_grows
			updateStep(percentOnly,
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "when_cost_grows"), knownvalue.ObjectExact(map[string]knownvalue.Check{
					"match":            knownvalue.Null(),
					"percent_increase": knownvalue.Int64Exact(10),
					"min_increase":     knownvalue.Null(),
					"within_hours":     knownvalue.Null(),
				})),
			),
			testAccCustomerNotificationDriftCheck(percentOnly),
			// Step 5: Clear the when_cost_grows branch, keep every_hours
			updateStep(everyHoursOnly,
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "every_hours"), knownvalue.Int64Exact(12)),
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "when_cost_grows"), knownvalue.Null()),
			),
			testAccCustomerNotificationDriftCheck(everyHoursOnly),
			// Step 7: Swap every_hours for a growth-only rule
			updateStep(growthOnly,
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "every_hours"), knownvalue.Null()),
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "when_cost_grows", "min_increase", "amount"), knownvalue.StringExact("250.00")),
			),
			testAccCustomerNotificationDriftCheck(growthOnly),
			// Step 9: Remove repeat entirely
			updateStep(noRepeat,
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat"), knownvalue.Null()),
			),
			testAccCustomerNotificationDriftCheck(noRepeat),
			// Step 11: Set repeat again on an existing notification
			updateStep(full,
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "every_hours"), knownvalue.Int64Exact(12)),
				statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, testAccCustomerNotificationPath("repeat", "when_cost_grows", "match"), knownvalue.StringExact("and")),
			),
			testAccCustomerNotificationDriftCheck(full),
		},
	})
}

// TestAccCustomerNotification_Destinations covers the destinations object: an
// email written in mixed case (the API lowercases it) keeps the configured
// spelling, and Slack channels are added, reordered and removed in place, up to
// a Slack-only notification. The API keeps list order as sent. Emails must
// belong to invited users, so the order checks use the two Slack channel IDs
// TEST_SLACK_CHAN and TEST_SLACK_WORKSPACE_CHAN.
func TestAccCustomerNotification_Destinations(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-dest")
	slackA := os.Getenv("TEST_SLACK_CHAN")
	slackB := os.Getenv("TEST_SLACK_WORKSPACE_CHAN")

	email := strings.ToUpper(testUser())
	quote := func(values []string) string {
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = fmt.Sprintf("%q", v)
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	}
	dest := func(emails, slack []string) string {
		return fmt.Sprintf(`{ emails = %s, slack_channels = %s, teams_channels = [] }`, quote(emails), quote(slack))
	}
	cfg := func(destinations string) string {
		return testAccCustomerNotificationConfigWithDestinations(rName, rName, destinations, testAccCustomerNotificationMinimalSettings)
	}
	listIs := func(attribute string, values ...string) statecheck.StateCheck {
		checks := make([]knownvalue.Check, len(values))
		for i, v := range values {
			checks[i] = knownvalue.StringExact(v)
		}
		return statecheck.ExpectKnownValue(testAccCustomerNotificationAddr, tfjsonpath.New("destinations").AtMapKey(attribute), knownvalue.ListExact(checks))
	}
	updateStep := func(destinations string, checks ...statecheck.StateCheck) resource.TestStep {
		return resource.TestStep{
			Config: cfg(destinations),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(testAccCustomerNotificationAddr, plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: checks,
		}
	}

	steps := []resource.TestStep{
		// The API stores the lowercase form; state keeps the configured one
		{
			Config:            cfg(dest([]string{email}, nil)),
			ConfigStateChecks: []statecheck.StateCheck{listIs("emails", email), listIs("slack_channels")},
		},
		testAccCustomerNotificationDriftCheck(cfg(dest([]string{email}, nil))),
	}
	if slackA != "" && slackB != "" {
		steps = append(steps,
			updateStep(dest([]string{email}, []string{slackA}), listIs("slack_channels", slackA)),
			testAccCustomerNotificationDriftCheck(cfg(dest([]string{email}, []string{slackA}))),
			updateStep(dest([]string{email}, []string{slackA, slackB}), listIs("slack_channels", slackA, slackB)),
			testAccCustomerNotificationDriftCheck(cfg(dest([]string{email}, []string{slackA, slackB}))),
			// Reversed order is kept
			updateStep(dest([]string{email}, []string{slackB, slackA}), listIs("slack_channels", slackB, slackA)),
			testAccCustomerNotificationDriftCheck(cfg(dest([]string{email}, []string{slackB, slackA}))),
			// Slack only: the unused email list is an empty list, not null
			updateStep(dest(nil, []string{slackB, slackA}), listIs("emails"), listIs("slack_channels", slackB, slackA)),
			testAccCustomerNotificationDriftCheck(cfg(dest(nil, []string{slackB, slackA}))),
		)
	}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    steps,
	})
}

func TestAccCustomerNotification_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-imp")

	// Amounts use the API's two-decimal spelling, so the imported state matches
	// the config exactly.
	cfg := testAccCustomerNotificationConfig(rName, rName, `
      min_severity    = "critical"
      min_excess_cost = { amount = "1000000.00", currency = "USD" }
      allocations     = { mode = "include", ids = [doit_allocation.a.id, doit_allocation.b.id] }
      repeat = {
        every_hours = 24
        when_cost_grows = {
          match            = "or"
          percent_increase = 25
          min_increase     = { amount = "500.00", currency = "USD" }
        }
      }
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
			// Drift check after import
			testAccCustomerNotificationDriftCheck(cfg),
		},
	})
}

// An amount written without decimals imports as the API's "1000000.00"
// spelling, so the first apply after import updates it to the configured
// spelling. From then on the values are equal and refresh keeps the configured
// spelling. The import block brings the notification created in step 1 under a
// second address; destroying both is safe because delete treats 404 as success.
func TestAccCustomerNotification_ImportAmountSpelling(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-imp2")
	settings := `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	cfg := testAccCustomerNotificationConfig(rName, rName, settings)
	withImport := cfg + fmt.Sprintf(`
import {
  to = doit_customer_notification.imported
  id = doit_customer_notification.this.id
}

resource "doit_customer_notification" "imported" {
  name         = %q
  destinations = doit_customer_notification.this.destinations
  settings = {
    cost_anomalies = {
%s
    }
  }
}
`, rName, settings)

	const importedAddr = "doit_customer_notification.imported"

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			// Import plus one in-place update of the amount spelling
			{
				Config: withImport,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(importedAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(importedAddr, testAccCustomerNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("1000000")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(importedAddr, testAccCustomerNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("1000000")),
				},
			},
			testAccCustomerNotificationDriftCheck(withImport),
		},
	})
}

// TestAccCustomerNotification_Disappears verifies that Terraform correctly handles
// a notification deleted outside of Terraform.
func TestAccCustomerNotification_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-gone")
	cfg := testAccCustomerNotificationConfig(rName, rName, testAccCustomerNotificationMinimalSettings)

	var id, etag string

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources[testAccCustomerNotificationAddr]
					if !ok {
						return fmt.Errorf("resource %s not found in state", testAccCustomerNotificationAddr)
					}
					id = rs.Primary.ID
					etag = rs.Primary.Attributes["etag"]
					return nil
				},
			},
			// Delete via the API, then verify Terraform plans to recreate it
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteCustomerNotificationWithResponse(t.Context(), id, &models.DeleteCustomerNotificationParams{
						IfMatch:        etag,
						IdempotencyKey: acctest.RandString(32),
					})
					if err != nil {
						t.Fatalf("Failed to delete customer notification via API: %v", err)
					}
					if resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccCustomerNotification_Validation checks that settings the API rejects with
// 422 fail at plan time instead.
func TestAccCustomerNotification_Validation(t *testing.T) {
	base := `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	cases := map[string]struct {
		settings     string
		destinations string // defaults to one email
		extra        string // config outside the notification, e.g. a source of unknown values
		err          string
	}{
		"non_usd_currency": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "EUR" }
      allocations     = { mode = "all", ids = [] }
`,
			err: `value must be\s+one\s+of`,
		},
		"fractional_amount": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100.50", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`,
			err: `whole\s+number\s+of\s+US\s+dollars`,
		},
		"fractional_min_increase": {
			settings: base + `
      repeat = { when_cost_grows = { min_increase = { amount = "1.5", currency = "USD" } } }
`,
			err: `whole\s+number\s+of\s+US\s+dollars`,
		},
		"no_destinations": {
			settings:     base,
			destinations: `{ emails = [], slack_channels = [], teams_channels = [] }`,
			err:          `No\s+Destinations`,
		},
		"duplicate_emails_ignoring_case": {
			settings:     base,
			destinations: `{ emails = ["a@example.com", "A@Example.com"], slack_channels = [], teams_channels = [] }`,
			err:          `Duplicate\s+Destination`,
		},
		"duplicate_slack_channels": {
			settings:     base,
			destinations: `{ emails = [], slack_channels = ["C0123", "C0123"], teams_channels = [] }`,
			err:          `Duplicate\s+Destination`,
		},
		"padded_email": {
			settings:     base,
			destinations: `{ emails = [" a@example.com"], slack_channels = [], teams_channels = [] }`,
			err:          `leading\s+or\s+trailing\s+whitespace`,
		},
		"missing_destination_list": {
			settings:     base,
			destinations: `{ emails = ["a@example.com"], slack_channels = [] }`,
			err:          `teams_channels`,
		},
		"all_with_ids": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "all", ids = ["alloc-a"] }
`,
			err: `Invalid Allocation IDs`,
		},
		"include_without_ids": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "include", ids = [] }
`,
			err: `Missing Allocation IDs`,
		},
		"duplicate_ids": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "exclude", ids = ["alloc-a", "alloc-a"] }
`,
			err: `Duplicate Allocation ID`,
		},
		"empty_repeat": {
			settings: base + `
      repeat = {}
`,
			err: `Empty Repeat\s+Configuration`,
		},
		"growth_without_threshold": {
			settings: base + `
      repeat = { when_cost_grows = { within_hours = 3 } }
`,
			err: `Missing Cost Growth\s+Threshold`,
		},
		"both_thresholds_without_match": {
			settings: base + `
      repeat = {
        when_cost_grows = {
          percent_increase = 5
          min_increase     = { amount = "10", currency = "USD" }
        }
      }
`,
			err: `Missing Match`,
		},
		"every_hours_out_of_range": {
			settings: base + `
      repeat = { every_hours = 25 }
`,
			err: `every_hours`,
		},
		"min_excess_cost_above_maximum": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000001", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`,
			err: `at\s+most\s+1000000000`,
		},
		"min_increase_zero": {
			settings: base + `
      repeat = { when_cost_grows = { min_increase = { amount = "0.00", currency = "USD" } } }
`,
			err: `greater\s+than\s+0`,
		},
		"min_increase_above_maximum": {
			settings: base + `
      repeat = { when_cost_grows = { min_increase = { amount = "1000000001", currency = "USD" } } }
`,
			err: `at\s+most\s+1000000000`,
		},
		"null_allocation_id": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "include", ids = [null] }
`,
			err: `Invalid Allocation ID`,
		},
		"empty_allocation_id": {
			settings: `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "include", ids = ["alloc-a", ""] }
`,
			err: `Invalid Allocation ID`,
		},
		// match is unknown at plan time, but no value of it can make a rule
		// without thresholds valid.
		"unknown_match_without_threshold": {
			extra: `
resource "terraform_data" "match" {
  input = "or"
}
`,
			settings: base + `
      repeat = { when_cost_grows = { match = terraform_data.match.output, within_hours = 3 } }
`,
			err: `Missing Cost Growth\s+Threshold`,
		},
	}

	for name, tc := range cases {
		if tc.destinations == "" {
			tc.destinations = `{ emails = ["a@example.com"], slack_channels = [], teams_channels = [] }`
		}
		t.Run(name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: tc.extra + fmt.Sprintf(`
resource "doit_customer_notification" "this" {
  name         = "validation"
  destinations = %s
  settings = {
    cost_anomalies = {
%s
    }
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

const testAccCustomerNotificationMinimalSettings = `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`

// testAccCustomerNotificationConfig returns two allocations the notification
// can scope to (only custom single allocations are accepted) and the
// notification itself, delivered by email to TEST_USER, with costAnomalies
// spliced into settings.cost_anomalies.
func testAccCustomerNotificationConfig(rName, name, costAnomalies string) string {
	return testAccCustomerNotificationConfigWithDestinations(rName, name,
		fmt.Sprintf(`{ emails = [%q], slack_channels = [], teams_channels = [] }`, testUser()), costAnomalies)
}

func testAccCustomerNotificationConfigWithDestinations(rName, name, destinations, costAnomalies string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "a" {
  name        = "%[1]s-a"
  description = "customer notification test allocation"
  rule = {
    formula    = "A"
    components = [{ key = "country", mode = "is", type = "fixed", values = ["JP"] }]
  }
}

resource "doit_allocation" "b" {
  name        = "%[1]s-b"
  description = "customer notification test allocation"
  rule = {
    formula    = "A"
    components = [{ key = "country", mode = "is", type = "fixed", values = ["DE"] }]
  }
}

resource "doit_customer_notification" "this" {
  name         = %[2]q
  destinations = %[3]s
  settings = {
    cost_anomalies = {
%[4]s
    }
  }
}
`, rName, name, destinations, costAnomalies)
}

func testAccCustomerNotificationDriftCheck(cfg string) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

func testAccCheckCustomerNotificationDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getAPIClient(t)
		ctx := t.Context()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "doit_customer_notification" {
				continue
			}
			resp, err := client.GetCustomerNotificationWithResponse(ctx, rs.Primary.ID)
			if err != nil {
				return fmt.Errorf("error checking customer notification %s: %w", rs.Primary.ID, err)
			}
			if resp.StatusCode() != 404 {
				return fmt.Errorf("customer notification %s still exists (status %d)", rs.Primary.ID, resp.StatusCode())
			}
		}
		return nil
	}
}
