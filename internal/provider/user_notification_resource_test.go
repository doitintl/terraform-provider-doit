package provider_test

import (
	"fmt"
	"regexp"
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

const testAccUserNotificationAddr = "doit_user_notification.this"

// The notifications are created on TEST_USER. Every test uses a very high
// min_excess_cost so the user is not emailed about real anomalies.

// testAccUserNotificationPath returns a fresh path under settings.cost_anomalies.
// Paths are built per call: deriving several paths from one shared prefix can
// alias its backing array and overwrite earlier paths.
func testAccUserNotificationPath(keys ...string) tfjsonpath.Path {
	p := tfjsonpath.New("settings").AtMapKey("cost_anomalies")
	for _, k := range keys {
		p = p.AtMapKey(k)
	}
	return p
}

func TestAccUserNotification_Lifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un")

	// Amounts are written without decimals on purpose: the API answers with
	// "1000000.00", and state must keep the configured spelling.
	all := testAccUserNotificationConfig(rName, rName, `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`)
	// b before a: the API keeps the configured order.
	include := testAccUserNotificationConfig(rName, rName+"-renamed", `
      min_severity    = "critical"
      min_excess_cost = { amount = "2000000.00", currency = "USD" }
      allocations     = { mode = "include", ids = [doit_allocation.b.id, doit_allocation.a.id] }
`)
	exclude := testAccUserNotificationConfig(rName, rName+"-renamed", `
      min_severity    = "critical"
      min_excess_cost = { amount = "2000000.00", currency = "USD" }
      allocations     = { mode = "exclude", ids = [doit_allocation.a.id] }
`)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with mode "all" and no repeat
			{
				Config: all,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("create_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("update_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("destinations"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"type":    knownvalue.StringExact("email"),
							"address": knownvalue.StringRegexp(regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(testUser()) + `$`)),
						}),
					})),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("1000000")),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("allocations", "ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat"), knownvalue.Null()),
				},
			},
			// Step 2: Drift check
			testAccUserNotificationDriftCheck(all),
			// Step 3: Update name, severity, amount and allocations in place
			{
				Config: include,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-renamed")),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("min_severity"), knownvalue.StringExact("critical")),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("2000000.00")),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("allocations", "mode"), knownvalue.StringExact("include")),
					statecheck.CompareValuePairs(
						testAccUserNotificationAddr, testAccUserNotificationPath("allocations", "ids").AtSliceIndex(0),
						"doit_allocation.b", tfjsonpath.New("id"), compare.ValuesSame()),
					statecheck.CompareValuePairs(
						testAccUserNotificationAddr, testAccUserNotificationPath("allocations", "ids").AtSliceIndex(1),
						"doit_allocation.a", tfjsonpath.New("id"), compare.ValuesSame()),
				},
			},
			// Step 4: Drift check (order of ids is kept)
			testAccUserNotificationDriftCheck(include),
			// Step 5: Switch to exclude with a shorter list
			{
				Config: exclude,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("allocations", "ids"), knownvalue.ListSizeExact(1)),
				},
			},
			// Step 6: Drift check
			testAccUserNotificationDriftCheck(exclude),
			// Step 7: Back to mode "all" with an empty list
			{
				Config: all,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("allocations", "ids"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Step 8: Drift check
			testAccUserNotificationDriftCheck(all),
		},
	})
}

// TestAccUserNotification_Repeat covers clearing repeat in steps: single
// leaves, the when_cost_grows branch, the whole block, and setting it again.
func TestAccUserNotification_Repeat(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-rep")

	base := `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	full := testAccUserNotificationConfig(rName, rName, base+`
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
	percentOnly := testAccUserNotificationConfig(rName, rName, base+`
      repeat = {
        every_hours = 12
        when_cost_grows = {
          percent_increase = 10
        }
      }
`)
	everyHoursOnly := testAccUserNotificationConfig(rName, rName, base+`
      repeat = {
        every_hours = 12
      }
`)
	growthOnly := testAccUserNotificationConfig(rName, rName, base+`
      repeat = {
        when_cost_grows = {
          min_increase = { amount = "250.00", currency = "USD" }
        }
      }
`)
	noRepeat := testAccUserNotificationConfig(rName, rName, base)

	updateStep := func(cfg string, checks ...statecheck.StateCheck) resource.TestStep {
		return resource.TestStep{
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(testAccUserNotificationAddr, plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: checks,
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with every resend control set
			{
				Config: full,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "every_hours"), knownvalue.Int64Exact(12)),
					statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "when_cost_grows"), knownvalue.ObjectExact(map[string]knownvalue.Check{
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
			testAccUserNotificationDriftCheck(full),
			// Step 3: Clear single leaves inside when_cost_grows
			updateStep(percentOnly,
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "when_cost_grows"), knownvalue.ObjectExact(map[string]knownvalue.Check{
					"match":            knownvalue.Null(),
					"percent_increase": knownvalue.Int64Exact(10),
					"min_increase":     knownvalue.Null(),
					"within_hours":     knownvalue.Null(),
				})),
			),
			testAccUserNotificationDriftCheck(percentOnly),
			// Step 5: Clear the when_cost_grows branch, keep every_hours
			updateStep(everyHoursOnly,
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "every_hours"), knownvalue.Int64Exact(12)),
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "when_cost_grows"), knownvalue.Null()),
			),
			testAccUserNotificationDriftCheck(everyHoursOnly),
			// Step 7: Swap every_hours for a growth-only rule
			updateStep(growthOnly,
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "every_hours"), knownvalue.Null()),
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "when_cost_grows", "min_increase", "amount"), knownvalue.StringExact("250.00")),
			),
			testAccUserNotificationDriftCheck(growthOnly),
			// Step 9: Remove repeat entirely
			updateStep(noRepeat,
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat"), knownvalue.Null()),
			),
			testAccUserNotificationDriftCheck(noRepeat),
			// Step 11: Set repeat again on an existing notification
			updateStep(full,
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "every_hours"), knownvalue.Int64Exact(12)),
				statecheck.ExpectKnownValue(testAccUserNotificationAddr, testAccUserNotificationPath("repeat", "when_cost_grows", "match"), knownvalue.StringExact("and")),
			),
			testAccUserNotificationDriftCheck(full),
		},
	})
}

func TestAccUserNotification_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-imp")

	// Amounts use the API's two-decimal spelling, so the imported state matches
	// the config exactly.
	cfg := testAccUserNotificationConfig(rName, rName, `
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
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				ResourceName:            testAccUserNotificationAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
				ImportStateIdFunc:       testAccUserNotificationImportID,
			},
			// Drift check after import
			testAccUserNotificationDriftCheck(cfg),
		},
	})
}

// An amount written without decimals imports as the API's "1000000.00"
// spelling, so the first apply after import updates it to the configured
// spelling. From then on the values are equal and refresh keeps the configured
// spelling. The import block brings the notification created in step 1 under a
// second address; destroying both is safe because delete treats 404 as success.
func TestAccUserNotification_ImportAmountSpelling(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-imp2")
	settings := `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	cfg := testAccUserNotificationConfig(rName, rName, settings)
	withImport := cfg + fmt.Sprintf(`
import {
  to = doit_user_notification.imported
  id = "${doit_user_notification.this.user_id}/${doit_user_notification.this.id}"
}

resource "doit_user_notification" "imported" {
  user_id = doit_user_notification.this.user_id
  name    = %q
  settings = {
    cost_anomalies = {
%s
    }
  }
}
`, rName, settings)

	const importedAddr = "doit_user_notification.imported"

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
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
						plancheck.ExpectKnownValue(importedAddr, testAccUserNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("1000000")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(importedAddr, testAccUserNotificationPath("min_excess_cost", "amount"), knownvalue.StringExact("1000000")),
				},
			},
			testAccUserNotificationDriftCheck(withImport),
		},
	})
}

func TestAccUserNotification_ImportInvalidID(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-badimp")

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: testAccUserNotificationConfig(rName, rName, testAccUserNotificationMinimalSettings)},
			{
				ResourceName:  testAccUserNotificationAddr,
				ImportState:   true,
				ImportStateId: "not-a-composite-id",
				ExpectError:   regexp.MustCompile(`user_id/notification_id`),
			},
		},
	})
}

// TestAccUserNotification_Disappears verifies that Terraform correctly handles
// a notification deleted outside of Terraform.
func TestAccUserNotification_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-gone")
	cfg := testAccUserNotificationConfig(rName, rName, testAccUserNotificationMinimalSettings)

	var userID, id, etag string

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckUserNotificationDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources[testAccUserNotificationAddr]
					if !ok {
						return fmt.Errorf("resource %s not found in state", testAccUserNotificationAddr)
					}
					userID = rs.Primary.Attributes["user_id"]
					id = rs.Primary.ID
					etag = rs.Primary.Attributes["etag"]
					return nil
				},
			},
			// Delete via the API, then verify Terraform plans to recreate it
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteUserNotificationWithResponse(t.Context(), userID, id, &models.DeleteUserNotificationParams{
						IfMatch:        etag,
						IdempotencyKey: acctest.RandString(32),
					})
					if err != nil {
						t.Fatalf("Failed to delete user notification via API: %v", err)
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

// TestAccUserNotification_Validation checks that settings the API rejects with
// 422 fail at plan time instead.
func TestAccUserNotification_Validation(t *testing.T) {
	base := `
      min_severity    = "warning"
      min_excess_cost = { amount = "100", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`
	cases := map[string]struct {
		settings string
		extra    string // config outside the notification, e.g. a source of unknown values
		err      string
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
		t.Run(name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: tc.extra + fmt.Sprintf(`
resource "doit_user_notification" "this" {
  user_id = "user-id"
  name    = "validation"
  settings = {
    cost_anomalies = {
%s
    }
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

const testAccUserNotificationMinimalSettings = `
      min_severity    = "warning"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
`

// testAccUserNotificationConfig returns TEST_USER, two allocations the
// notification can scope to (only custom single allocations are accepted),
// and the notification itself with costAnomalies spliced into
// settings.cost_anomalies.
func testAccUserNotificationConfig(rName, name, costAnomalies string) string {
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

resource "doit_allocation" "b" {
  name        = "%[2]s-b"
  description = "user notification test allocation"
  rule = {
    formula    = "A"
    components = [{ key = "country", mode = "is", type = "fixed", values = ["DE"] }]
  }
}

resource "doit_user_notification" "this" {
  user_id = data.doit_users.test.users[0].id
  name    = %[3]q
  settings = {
    cost_anomalies = {
%[4]s
    }
  }
}
`, testUser(), rName, name, costAnomalies)
}

func testAccUserNotificationDriftCheck(cfg string) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

func testAccUserNotificationImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources[testAccUserNotificationAddr]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", testAccUserNotificationAddr)
	}
	return rs.Primary.Attributes["user_id"] + "/" + rs.Primary.ID, nil
}

func testAccCheckUserNotificationDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getAPIClient(t)
		ctx := t.Context()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "doit_user_notification" {
				continue
			}
			resp, err := client.GetUserNotificationWithResponse(ctx, rs.Primary.Attributes["user_id"], rs.Primary.ID)
			if err != nil {
				return fmt.Errorf("error checking user notification %s: %w", rs.Primary.ID, err)
			}
			if resp.StatusCode() != 404 {
				return fmt.Errorf("user notification %s still exists (status %d)", rs.Primary.ID, resp.StatusCode())
			}
		}
		return nil
	}
}
