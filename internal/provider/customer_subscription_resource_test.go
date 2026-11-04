package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testAccCustomerSubscriptionAddr = "doit_customer_subscription.this"

// The subscriptions are delivered to TEST_USER. Every schedule starts in 2030,
// so nobody receives a delivery while the tests run.

// testAccCustomerSubscriptionSchedule is a weekly schedule in UTC.
const testAccCustomerSubscriptionSchedule = `{
        time_zone  = "UTC"
        start_date = "2030-01-07T00:00:00Z"
        time       = "2030-01-07T08:00:00Z"
        frequency  = "Weekly"
      }`

// testAccCustomerSubscriptionReportEntry returns a report-subscription entry
// for doit_report.this. extra is spliced into the entry (e.g. attachments).
func testAccCustomerSubscriptionReportEntry(schedule, extra string) string {
	return fmt.Sprintf(`{
      type     = "report-subscription"
      source   = { id = doit_report.this.id }
      schedule = %s
%s
    }`, schedule, extra)
}

// testAccCustomerSubscriptionCommitmentEntry returns a commitment-manager
// entry for the first commitment of the tenant.
func testAccCustomerSubscriptionCommitmentEntry() string {
	return fmt.Sprintf(`{
      type     = "commitment-manager"
      source   = { id = data.doit_commitments.all.commitments[0].id }
      schedule = %s
    }`, testAccCustomerSubscriptionSchedule)
}

// testAccCustomerSubscriptionConfig returns a report the subscription can
// deliver, the tenant's commitments, and the subscription with the given
// recipients and subscription entries.
func testAccCustomerSubscriptionConfig(rName, recipients string, entries ...string) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
  name = "%[1]s-report"
  config = {
    metrics = [{
      type  = "basic"
      value = "cost"
    }]
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}

data "doit_commitments" "all" {}

resource "doit_customer_subscription" "this" {
  recipients    = %[2]s
  subscriptions = [
    %[3]s
  ]
}
`, rName, recipients, strings.Join(entries, ",\n    "))
}

func testAccCustomerSubscriptionEmailRecipients() string {
	return fmt.Sprintf(`{ emails = [%q], slack_channels = [] }`, testUser())
}

func testAccCustomerSubscriptionDriftCheck(cfg string) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

func testAccCheckCustomerSubscriptionDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getAPIClient(t)
		ctx := t.Context()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "doit_customer_subscription" {
				continue
			}
			resp, err := client.GetCustomerSubscriptionWithResponse(ctx, rs.Primary.ID)
			if err != nil {
				return fmt.Errorf("error checking customer subscription %s: %w", rs.Primary.ID, err)
			}
			if resp.StatusCode() != 404 {
				return fmt.Errorf("customer subscription %s still exists (status %d)", rs.Primary.ID, resp.StatusCode())
			}
		}
		return nil
	}
}

// testAccCustomerSubscriptionReportCheck checks the single report entry.
func testAccCustomerSubscriptionReportCheck(checks map[string]knownvalue.Check) statecheck.StateCheck {
	checks["type"] = knownvalue.StringExact("report-subscription")
	return statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("subscriptions"),
		knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectPartial(checks)}))
}

func testAccCustomerSubscriptionPdfSettings(showFilters, includeTable, includeAnnotations bool) knownvalue.Check {
	return knownvalue.ObjectExact(map[string]knownvalue.Check{
		"show_filters":        knownvalue.Bool(showFilters),
		"include_table":       knownvalue.Bool(includeTable),
		"include_annotations": knownvalue.Bool(includeAnnotations),
	})
}

func TestAccCustomerSubscription_Lifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub")

	// csv before pdf: the API keeps the configured order.
	initial := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, `      attachments = ["csv", "pdf"]`))
	updated := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(`{
        time_zone  = "Europe/Berlin"
        start_date = "2030-02-01T00:00:00Z"
        time       = "2030-02-01T06:30:00Z"
        frequency  = "Monthly"
      }`, `      attachments  = ["pdf"]
      pdf_settings = { show_filters = false, include_table = true, include_annotations = false }`))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("created_by"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-report (Report Subscription)")),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("recipients"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"emails":         knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(testUser())}),
						"slack_channels": knownvalue.ListExact([]knownvalue.Check{}),
					})),
					testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{
						"source":       knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact(rName + "-report")}),
						"attachments":  knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("csv"), knownvalue.StringExact("pdf")}),
						"pdf_settings": testAccCustomerSubscriptionPdfSettings(true, true, true),
						"permissions":  knownvalue.ObjectPartial(map[string]knownvalue.Check{"can_delete": knownvalue.Bool(true)}),
						"schedule": knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"frequency": knownvalue.StringExact("Weekly"),
							"next_at":   knownvalue.NotNull(),
						}),
					}),
				},
			},
			// Step 2: Drift check
			testAccCustomerSubscriptionDriftCheck(initial),
			// Step 3: Update schedule, attachments and PDF settings in place
			{
				Config: updated,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{
						"attachments":  knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("pdf")}),
						"pdf_settings": testAccCustomerSubscriptionPdfSettings(false, true, false),
						"schedule": knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"frequency": knownvalue.StringExact("Monthly"),
							"time_zone": knownvalue.StringExact("Europe/Berlin"),
						}),
					}),
				},
			},
			// Step 4: Drift check
			testAccCustomerSubscriptionDriftCheck(updated),
		},
	})
}

// TestAccCustomerSubscription_SetMembership adds a commitment entry next to the
// report entry, then removes the report entry: an update sends the full set.
func TestAccCustomerSubscription_SetMembership(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-set")

	reportOnly := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""))
	both := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""),
		testAccCustomerSubscriptionCommitmentEntry())
	commitmentOnly := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionCommitmentEntry())

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: reportOnly},
			testAccCustomerSubscriptionDriftCheck(reportOnly),
			{
				Config: both,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("subscriptions"), knownvalue.SetSizeExact(2)),
					// The name follows the type that sorts first.
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("name"), knownvalue.StringRegexp(regexp.MustCompile(`\(Commitment Subscription\)$`))),
				},
			},
			testAccCustomerSubscriptionDriftCheck(both),
			{
				Config: commitmentOnly,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("subscriptions"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"type":         knownvalue.StringExact("commitment-manager"),
							"attachments":  knownvalue.ListExact([]knownvalue.Check{}),
							"pdf_settings": knownvalue.Null(),
						}),
					})),
				},
			},
			testAccCustomerSubscriptionDriftCheck(commitmentOnly),
		},
	})
}

// TestAccCustomerSubscription_OmittedOptionalComputed covers attachments and
// pdf_settings left out of a report entry: the API fills in [] and all true,
// and later plans must stay empty, also after a change to another attribute
// of the same entry or of the resource.
func TestAccCustomerSubscription_OmittedOptionalComputed(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-oc")

	omitted := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""))
	otherTime := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(strings.Replace(testAccCustomerSubscriptionSchedule, "T08:00:00Z", "T09:15:00Z", 1), ""))

	defaults := testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{
		"attachments":  knownvalue.ListExact([]knownvalue.Check{}),
		"pdf_settings": testAccCustomerSubscriptionPdfSettings(true, true, true),
	})

	steps := []resource.TestStep{
		{Config: omitted, ConfigStateChecks: []statecheck.StateCheck{defaults}},
		testAccCustomerSubscriptionDriftCheck(omitted),
		{
			Config: otherTime,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{defaults},
		},
		testAccCustomerSubscriptionDriftCheck(otherTime),
	}

	// A change outside the set: the entry itself is unchanged.
	if slack := testSlackChannel(); slack != "" {
		withSlack := testAccCustomerSubscriptionConfig(rName, fmt.Sprintf(`{ emails = [%q], slack_channels = [%q] }`, testUser(), slack),
			testAccCustomerSubscriptionReportEntry(strings.Replace(testAccCustomerSubscriptionSchedule, "T08:00:00Z", "T09:15:00Z", 1), ""))
		steps = append(steps,
			resource.TestStep{
				Config: withSlack,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					defaults,
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("recipients").AtMapKey("slack_channels"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(slack)})),
				},
			},
			testAccCustomerSubscriptionDriftCheck(withSlack),
		)
	}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    steps,
	})
}

// TestAccCustomerSubscription_ClearAttachments removes attachments from the
// configuration, which clears them (Category A), and sets them again.
func TestAccCustomerSubscription_ClearAttachments(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-clr")

	withAttachments := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, `      attachments = ["pdf", "csv"]`))
	without := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""))

	attachments := func(values ...string) statecheck.StateCheck {
		checks := make([]knownvalue.Check, len(values))
		for i, v := range values {
			checks[i] = knownvalue.StringExact(v)
		}
		return testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{"attachments": knownvalue.ListExact(checks)})
	}
	update := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction(testAccCustomerSubscriptionAddr, plancheck.ResourceActionUpdate),
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with attachments
			{Config: withAttachments, ConfigStateChecks: []statecheck.StateCheck{attachments("pdf", "csv")}},
			// Step 2: Drift check
			testAccCustomerSubscriptionDriftCheck(withAttachments),
			// Step 3: Remove attachments from the configuration, which clears them
			{Config: without, ConfigPlanChecks: update, ConfigStateChecks: []statecheck.StateCheck{attachments()}},
			// Step 4: Drift check (the cleared list is stable)
			testAccCustomerSubscriptionDriftCheck(without),
			// Step 5: Set them again
			{Config: withAttachments, ConfigPlanChecks: update, ConfigStateChecks: []statecheck.StateCheck{attachments("pdf", "csv")}},
			// Step 6: Drift check
			testAccCustomerSubscriptionDriftCheck(withAttachments),
		},
	})
}

// TestAccCustomerSubscription_CustomFrequency switches a Custom frequency back
// to Weekly, which drops custom_period_amount and custom_period_unit.
func TestAccCustomerSubscription_CustomFrequency(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-cust")

	custom := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(`{
        time_zone            = "UTC"
        start_date           = "2030-01-07T00:00:00Z"
        time                 = "2030-01-07T08:00:00Z"
        frequency            = "Custom"
        custom_period_amount = 2
        custom_period_unit   = "Week"
      }`, ""))
	weekly := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: custom,
				ConfigStateChecks: []statecheck.StateCheck{
					testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{
						"schedule": knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"frequency":            knownvalue.StringExact("Custom"),
							"custom_period_amount": knownvalue.Int64Exact(2),
							"custom_period_unit":   knownvalue.StringExact("Week"),
						}),
					}),
				},
			},
			testAccCustomerSubscriptionDriftCheck(custom),
			{
				Config: weekly,
				ConfigStateChecks: []statecheck.StateCheck{
					testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{
						"schedule": knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"frequency":            knownvalue.StringExact("Weekly"),
							"custom_period_amount": knownvalue.Null(),
							"custom_period_unit":   knownvalue.Null(),
						}),
					}),
				},
			},
			testAccCustomerSubscriptionDriftCheck(weekly),
		},
	})
}

// TestAccCustomerSubscription_Normalization uses values the API normalizes: an
// uppercase email (stored lowercase) and timestamps with offsets (returned in
// UTC). State keeps the configured spelling and plans stay empty.
func TestAccCustomerSubscription_Normalization(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-norm")

	cfg := testAccCustomerSubscriptionConfig(rName, fmt.Sprintf(`{ emails = [%q], slack_channels = [] }`, strings.ToUpper(testUser())),
		testAccCustomerSubscriptionReportEntry(`{
        time_zone  = "America/Los_Angeles"
        start_date = "2030-01-07T00:00:00-08:00"
        time       = "2030-01-07T10:00:00+02:00"
        frequency  = "Daily"
      }`, ""))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("recipients").AtMapKey("emails"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(strings.ToUpper(testUser()))})),
					testAccCustomerSubscriptionReportCheck(map[string]knownvalue.Check{
						"schedule": knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"start_date": knownvalue.StringExact("2030-01-07T00:00:00-08:00"),
							"time":       knownvalue.StringExact("2030-01-07T10:00:00+02:00"),
						}),
					}),
				},
			},
			// Refresh keeps the configured spelling of the same values.
			testAccCustomerSubscriptionDriftCheck(cfg),
		},
	})
}

func TestAccCustomerSubscription_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-imp")

	// Values use the API's spelling, so the imported state matches the config.
	cfg := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, `      attachments = ["pdf"]`),
		testAccCustomerSubscriptionCommitmentEntry())

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				ResourceName:            testAccCustomerSubscriptionAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			// Drift check after import
			testAccCustomerSubscriptionDriftCheck(cfg),
		},
	})
}

// TestAccCustomerSubscription_ImportAlertRejected imports an alert ID, which
// the subscriptions API also returns, and expects a clear error.
func TestAccCustomerSubscription_ImportAlertRejected(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-alert")

	alert := fmt.Sprintf(`
resource "doit_alert" "this" {
  name = %q
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "year"
    value         = 1000000000
    operator      = "gt"
  }
}
`, rName)
	withImport := alert + fmt.Sprintf(`
import {
  to = doit_customer_subscription.imported
  id = doit_alert.this.id
}

resource "doit_customer_subscription" "imported" {
  recipients    = %s
  subscriptions = [{
    type     = "report-subscription"
    source   = { id = "unused" }
    schedule = %s
  }]
}
`, testAccCustomerSubscriptionEmailRecipients(), testAccCustomerSubscriptionSchedule)

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: alert},
			{
				Config:      withImport,
				ExpectError: regexp.MustCompile(`Unsupported Subscription Type`),
			},
		},
	})
}

// TestAccCustomerSubscription_Dashboard covers a dashboard entry. The API has
// no endpoint that lists dashboards, so the ID comes from TEST_DASHBOARD_ID.
func TestAccCustomerSubscription_Dashboard(t *testing.T) {
	dashboardID := os.Getenv("TEST_DASHBOARD_ID")
	if dashboardID == "" {
		t.Skip("TEST_DASHBOARD_ID is not set, skipping dashboard subscription test")
	}
	rName := acctest.RandomWithPrefix("tf-acc-csub-dash")

	cfg := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		fmt.Sprintf(`{
      type     = "dashboard-subscription"
      source   = { id = %q }
      schedule = %s
    }`, dashboardID, testAccCustomerSubscriptionSchedule),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""))

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("name"), knownvalue.StringRegexp(regexp.MustCompile(`\(Dashboard Subscription\)$`))),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionAddr, tfjsonpath.New("subscriptions"), knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"type":         knownvalue.StringExact("dashboard-subscription"),
							"source":       knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.NotNull()}),
							"attachments":  knownvalue.ListExact([]knownvalue.Check{}),
							"pdf_settings": knownvalue.Null(),
						}),
					})),
				},
			},
			testAccCustomerSubscriptionDriftCheck(cfg),
		},
	})
}

// TestAccCustomerSubscription_Disappears verifies that Terraform correctly handles
// a subscription deleted outside of Terraform.
func TestAccCustomerSubscription_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-gone")
	cfg := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""))

	var id, etag string

	resource.ParallelTest(t, resource.TestCase{
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources[testAccCustomerSubscriptionAddr]
					if !ok {
						return fmt.Errorf("resource %s not found in state", testAccCustomerSubscriptionAddr)
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
					resp, err := client.DeleteCustomerSubscriptionWithResponse(t.Context(), id, &models.DeleteCustomerSubscriptionParams{
						IfMatch:        etag,
						IdempotencyKey: acctest.RandString(32),
					})
					if err != nil {
						t.Fatalf("Failed to delete customer subscription via API: %v", err)
					}
					if resp.StatusCode() != 204 {
						t.Fatalf("Expected 204 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccCustomerSubscription_Validation checks that configurations the API
// rejects with 422 fail at plan time instead.
func TestAccCustomerSubscription_Validation(t *testing.T) {
	recipients := testAccCustomerSubscriptionEmailRecipients()
	cases := map[string]struct {
		recipients string
		entries    []string
		err        string
	}{
		"no_recipients": {
			recipients: `{ emails = [], slack_channels = [] }`,
			entries:    []string{testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, "")},
			err:        `No Destinations`,
		},
		"duplicate_type": {
			entries: []string{
				testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ""),
				testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, `      attachments = ["pdf"]`),
			},
			err: `Duplicate Subscription Type`,
		},
		"attachments_on_commitment": {
			entries: []string{fmt.Sprintf(`{
      type        = "commitment-manager"
      source      = { id = data.doit_commitments.all.commitments[0].id }
      schedule    = %s
      attachments = ["pdf"]
    }`, testAccCustomerSubscriptionSchedule)},
			err: `Attachments Not Allowed`,
		},
		"invalid_attachment": {
			entries: []string{testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, `      attachments = ["xlsx"]`)},
			err:     `Invalid Attribute Value Match`,
		},
		"custom_without_period": {
			entries: []string{testAccCustomerSubscriptionReportEntry(`{
        time_zone  = "UTC"
        start_date = "2030-01-07T00:00:00Z"
        time       = "2030-01-07T08:00:00Z"
        frequency  = "Custom"
      }`, "")},
			err: `Missing Custom Period`,
		},
		"period_without_custom": {
			entries: []string{testAccCustomerSubscriptionReportEntry(`{
        time_zone            = "UTC"
        start_date           = "2030-01-07T00:00:00Z"
        time                 = "2030-01-07T08:00:00Z"
        frequency            = "Weekly"
        custom_period_amount = 2
        custom_period_unit   = "Week"
      }`, "")},
			err: `Custom Period Not Allowed`,
		},
		"invalid_timestamp": {
			entries: []string{testAccCustomerSubscriptionReportEntry(`{
        time_zone  = "UTC"
        start_date = "2030-01-07"
        time       = "2030-01-07T08:00:00Z"
        frequency  = "Weekly"
      }`, "")},
			err: `Invalid RFC3339 Timestamp`,
		},
		"padded_email": {
			recipients: `{ emails = [" finops@example.com"], slack_channels = [] }`,
			entries:    []string{testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, "")},
			err:        `Invalid Value`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := tc.recipients
			if r == "" {
				r = recipients
			}
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config:      testAccCustomerSubscriptionConfig("tf-acc-csub-val", r, tc.entries...),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(tc.err),
					},
				},
			})
		})
	}
}
