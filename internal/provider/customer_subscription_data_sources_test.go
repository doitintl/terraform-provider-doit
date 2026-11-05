package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const (
	testAccCustomerSubscriptionDataSourceAddr  = "data.doit_customer_subscription.test"
	testAccCustomerSubscriptionsDataSourceAddr = "data.doit_customer_subscriptions.test"
)

// The organization has other subscriptions created by tests running in
// parallel, so the list tests find their subscription by ID and never assert a
// count or an index.

func testAccCustomerSubscriptionDataSourceConfig(cfg string) string {
	return cfg + `
data "doit_customer_subscription" "test" {
  id = doit_customer_subscription.this.id
}
`
}

// testAccCheckCustomerSubscriptionsListContains checks that the list data
// source holds the subscription of resourceAddr and that the entry has
// wantEntries nested subscriptions.
func testAccCheckCustomerSubscriptionsListContains(dataAddr, resourceAddr string, wantEntries int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ds, ok := s.RootModule().Resources[dataAddr]
		if !ok {
			return fmt.Errorf("data source %s not found", dataAddr)
		}
		res, ok := s.RootModule().Resources[resourceAddr]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceAddr)
		}
		attrs := ds.Primary.Attributes
		n, err := strconv.Atoi(attrs["items.#"])
		if err != nil {
			return fmt.Errorf("items.# = %q: %w", attrs["items.#"], err)
		}
		for i := range n {
			if attrs[fmt.Sprintf("items.%d.id", i)] != res.Primary.ID {
				continue
			}
			if got := attrs[fmt.Sprintf("items.%d.subscriptions.#", i)]; got != strconv.Itoa(wantEntries) {
				return fmt.Errorf("item %d has %s subscriptions, want %d", i, got, wantEntries)
			}
			if got, want := attrs[fmt.Sprintf("items.%d.etag", i)], res.Primary.Attributes["etag"]; got != want {
				return fmt.Errorf("item %d etag = %q, want %q", i, got, want)
			}
			return nil
		}
		return fmt.Errorf("subscription %s not found among %d items", res.Primary.ID, n)
	}
}

func TestAccCustomerSubscriptionDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-ds")
	// The API lowercases emails; the data source returns its spelling.
	email := strings.ToUpper(testUser())
	config := testAccCustomerSubscriptionDataSourceConfig(testAccCustomerSubscriptionConfig(rName,
		fmt.Sprintf(`{ emails = [%q], slack_channels = [] }`, email),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, `      attachments  = ["csv", "pdf"]
      pdf_settings = { show_filters = false, include_table = true, include_annotations = true }`)))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccCustomerSubscriptionDataSourceAddr, "id", testAccCustomerSubscriptionAddr, "id"),
					resource.TestCheckResourceAttrPair(testAccCustomerSubscriptionDataSourceAddr, "name", testAccCustomerSubscriptionAddr, "name"),
					resource.TestCheckResourceAttrPair(testAccCustomerSubscriptionDataSourceAddr, "etag", testAccCustomerSubscriptionAddr, "etag"),
					resource.TestCheckResourceAttrPair(testAccCustomerSubscriptionDataSourceAddr, "created_by", testAccCustomerSubscriptionAddr, "created_by"),
					resource.TestCheckResourceAttr(testAccCustomerSubscriptionDataSourceAddr, "recipients.emails.0", strings.ToLower(email)),
					resource.TestCheckResourceAttr(testAccCustomerSubscriptionDataSourceAddr, "recipients.slack_channels.#", "0"),
					resource.TestCheckResourceAttr(testAccCustomerSubscriptionDataSourceAddr, "subscriptions.#", "1"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionDataSourceAddr, tfjsonpath.New("subscriptions"),
						knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
							"type":         knownvalue.StringExact("report-subscription"),
							"source":       knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.NotNull()}),
							"attachments":  knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("csv"), knownvalue.StringExact("pdf")}),
							"pdf_settings": testAccCustomerSubscriptionPdfSettings(false, true, true),
							"permissions":  knownvalue.NotNull(),
							"schedule": knownvalue.ObjectExact(map[string]knownvalue.Check{
								"time_zone":            knownvalue.StringExact("UTC"),
								"start_date":           knownvalue.StringExact("2030-01-07T00:00:00Z"),
								"time":                 knownvalue.StringExact("2030-01-07T08:00:00Z"),
								"frequency":            knownvalue.StringExact("Weekly"),
								"next_at":              knownvalue.NotNull(),
								"custom_period_amount": knownvalue.Null(),
								"custom_period_unit":   knownvalue.Null(),
							}),
						})})),
				},
			},
			// Re-applying the same configuration must not change anything.
			testAccCustomerSubscriptionDriftCheck(config),
		},
	})
}

// TestAccCustomerSubscriptionDataSource_Dashboard checks that the entries the
// API does not give attachments or PDF settings read as [] and null.
func TestAccCustomerSubscriptionDataSource_Dashboard(t *testing.T) {
	dashboardID := os.Getenv("TEST_DASHBOARD_ID")
	if dashboardID == "" {
		t.Skip("TEST_DASHBOARD_ID is not set, skipping dashboard subscription test")
	}
	rName := acctest.RandomWithPrefix("tf-acc-csub-ds-dash")
	config := testAccCustomerSubscriptionDataSourceConfig(testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		fmt.Sprintf(`{
      type     = "dashboard-subscription"
      source   = { id = %q }
      schedule = %s
    }`, dashboardID, testAccCustomerSubscriptionSchedule),
		testAccCustomerSubscriptionCommitmentEntry()))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionDataSourceAddr, tfjsonpath.New("subscriptions"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"type":         knownvalue.StringExact("dashboard-subscription"),
							"attachments":  knownvalue.ListExact([]knownvalue.Check{}),
							"pdf_settings": knownvalue.Null(),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"type":         knownvalue.StringExact("commitment-manager"),
							"attachments":  knownvalue.ListExact([]knownvalue.Check{}),
							"pdf_settings": knownvalue.Null(),
						}),
					})),
				},
			},
		},
	})
}

func TestAccCustomerSubscriptionDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_customer_subscription" "test" {
  id = "tf-acc-does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`Error Reading Customer Subscription`),
			},
		},
	})
}

func TestAccCustomerSubscriptionsDataSource_List(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-ds-list")
	config := testAccCustomerSubscriptionConfig(rName, testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ``),
		testAccCustomerSubscriptionCommitmentEntry()) + `
data "doit_customer_subscriptions" "test" {
  depends_on = [doit_customer_subscription.this]
}
`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCustomerSubscriptionsListContains(testAccCustomerSubscriptionsDataSourceAddr, testAccCustomerSubscriptionAddr, 2),
					// Auto mode: the inputs stay unset.
					resource.TestCheckNoResourceAttr(testAccCustomerSubscriptionsDataSourceAddr, "max_results"),
					resource.TestCheckNoResourceAttr(testAccCustomerSubscriptionsDataSourceAddr, "page_token"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionsDataSourceAddr, tfjsonpath.New("row_count"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestAccCustomerSubscriptionsDataSource_Pagination creates two subscriptions,
// so that a page of one always has a next page.
func TestAccCustomerSubscriptionsDataSource_Pagination(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-csub-ds-page")
	first := testAccCustomerSubscriptionConfig(rName+"-a", testAccCustomerSubscriptionEmailRecipients(),
		testAccCustomerSubscriptionReportEntry(testAccCustomerSubscriptionSchedule, ``))
	// The second subscription gets its own report, and shares the commitments
	// data source of the first.
	second := strings.NewReplacer(
		`resource "doit_report" "this"`, `resource "doit_report" "that"`,
		`doit_report.this.id`, `doit_report.that.id`,
		`resource "doit_customer_subscription" "this"`, `resource "doit_customer_subscription" "that"`,
		`data "doit_commitments" "all" {}`, ``,
		rName+"-a", rName+"-b",
	).Replace(first)
	config := first + second + `
data "doit_customer_subscriptions" "paged" {
  max_results = 1
  depends_on  = [doit_customer_subscription.this, doit_customer_subscription.that]
}

data "doit_customer_subscriptions" "all" {
  depends_on = [doit_customer_subscription.this, doit_customer_subscription.that]
}
`
	const paged = "data.doit_customer_subscriptions.paged"
	const all = "data.doit_customer_subscriptions.all"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerSubscriptionDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					// One page of one item, with the token of the next page.
					resource.TestCheckResourceAttr(paged, "items.#", "1"),
					resource.TestCheckResourceAttr(paged, "max_results", "1"),
					resource.TestCheckResourceAttrSet(paged, "page_token"),
					// Auto mode fetches both of ours, however many pages that takes.
					testAccCheckCustomerSubscriptionsListContains(all, testAccCustomerSubscriptionAddr, 1),
					testAccCheckCustomerSubscriptionsListContains(all, "doit_customer_subscription.that", 1),
				),
			},
		},
	})
}

// testAccCustomerSubscriptionNonScheduledChecks checks the one entry of an
// alert or a budget: it has no schedule, attachments or PDF settings, and its
// source is the alert or budget itself.
func testAccCustomerSubscriptionNonScheduledChecks(entryType, resourceAddr string) []statecheck.StateCheck {
	return []statecheck.StateCheck{
		statecheck.ExpectKnownValue(testAccCustomerSubscriptionDataSourceAddr, tfjsonpath.New("subscriptions"),
			knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
				"type":         knownvalue.StringExact(entryType),
				"source":       knownvalue.ObjectPartial(map[string]knownvalue.Check{"id": knownvalue.NotNull()}),
				"schedule":     knownvalue.Null(),
				"attachments":  knownvalue.ListExact([]knownvalue.Check{}),
				"pdf_settings": knownvalue.Null(),
				"permissions":  knownvalue.NotNull(),
			})})),
		statecheck.CompareValuePairs(testAccCustomerSubscriptionDataSourceAddr, tfjsonpath.New("id"),
			resourceAddr, tfjsonpath.New("id"), compare.ValuesSame()),
	}
}

// TestAccCustomerSubscriptionDataSource_Alert reads an alert through the
// subscription data source: the subscriptions API shares its ID space with
// alerts, so the ID of a doit_alert is accepted.
func TestAccCustomerSubscriptionDataSource_Alert(t *testing.T) {
	n := acctest.RandInt()
	config := testAccAlertMinimalConfig(n) + `
data "doit_customer_subscription" "test" {
  id = doit_alert.this.id
}
`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: append(testAccCustomerSubscriptionNonScheduledChecks("alert", "doit_alert.this"),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionDataSourceAddr, tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-alert-minimal-%d", n)))),
			},
		},
	})
}

// TestAccCustomerSubscriptionDataSource_Budget is the budget counterpart of
// TestAccCustomerSubscriptionDataSource_Alert.
func TestAccCustomerSubscriptionDataSource_Budget(t *testing.T) {
	n := acctest.RandInt()
	config := testAccBudget(n) + `
data "doit_customer_subscription" "test" {
  id = doit_budget.this.id
}
`

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: append(testAccCustomerSubscriptionNonScheduledChecks("budget", "doit_budget.this"),
					statecheck.ExpectKnownValue(testAccCustomerSubscriptionDataSourceAddr, tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-min-%d", n)))),
			},
		},
	})
}
