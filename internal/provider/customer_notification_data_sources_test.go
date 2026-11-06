package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const (
	testAccCustomerNotificationDataSourceAddr  = "data.doit_customer_notification.test"
	testAccCustomerNotificationsDataSourceAddr = "data.doit_customer_notifications.test"
	testAccCustomerNotificationCopyAddr        = "doit_customer_notification.copy"
)

// The organization has other customer notifications created by tests running
// in parallel, so the list tests find their notification by name and never
// assert a count or an index.

func testAccCustomerNotificationDataSourceConfig(cfg string) string {
	return cfg + `
data "doit_customer_notification" "test" {
  id = doit_customer_notification.this.id
}
`
}

func TestAccCustomerNotificationDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-ds")
	config := testAccCustomerNotificationDataSourceConfig(
		testAccCustomerNotificationConfig(rName, rName, testAccCustomerNotificationMinimalSettings))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationDataSourceAddr, "id", testAccCustomerNotificationAddr, "id"),
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationDataSourceAddr, "name", testAccCustomerNotificationAddr, "name"),
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationDataSourceAddr, "etag", testAccCustomerNotificationAddr, "etag"),
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationDataSourceAddr, "create_time", testAccCustomerNotificationAddr, "create_time"),
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationDataSourceAddr, "update_time", testAccCustomerNotificationAddr, "update_time"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "destinations.emails.#", "1"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.min_severity", "warning"),
					// The data source reports the API's spelling of the amount, not the configured "1000000".
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.min_excess_cost.amount", "1000000.00"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.min_excess_cost.currency", "USD"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.allocations.mode", "all"),
					resource.TestCheckNoResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.every_hours"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, tfjsonpath.New("destinations").AtMapKey("slack_channels"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, tfjsonpath.New("destinations").AtMapKey("teams_channels"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, tfjsonpath.New("settings").AtMapKey("cost_anomalies").AtMapKey("allocations").AtMapKey("ids"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr,
						testAccCustomerNotificationPath("repeat"), knownvalue.Null()),
				},
			},
			testAccCustomerNotificationDriftCheck(config),
		},
	})
}

func TestAccCustomerNotificationDataSource_Repeat(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-ds-rep")
	config := testAccCustomerNotificationDataSourceConfig(testAccCustomerNotificationConfig(rName, rName, `
      min_severity    = "critical"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "include", ids = [doit_allocation.a.id, doit_allocation.b.id] }
      repeat = {
        every_hours = 12
        when_cost_grows = {
          match            = "and"
          percent_increase = 10
          min_increase     = { amount = "150", currency = "USD" }
          within_hours     = 6
        }
      }
`))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.min_severity", "critical"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.allocations.mode", "include"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.allocations.ids.#", "2"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.every_hours", "12"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.match", "and"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.percent_increase", "10"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.within_hours", "6"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.min_increase.amount", "150.00"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.min_increase.currency", "USD"),
				),
			},
			testAccCustomerNotificationDriftCheck(config),
		},
	})
}

// The API lowercases emails; the data source reports the lowercase spelling
// even when the resource keeps the configured one.
func TestAccCustomerNotificationDataSource_Destinations(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-ds-dest")
	slack := os.Getenv("TEST_SLACK_CHAN")
	if slack == "" {
		t.Skip("TEST_SLACK_CHAN not set")
	}
	email := strings.ToUpper(testUser())
	config := testAccCustomerNotificationDataSourceConfig(testAccCustomerNotificationConfigWithDestinations(rName, rName,
		fmt.Sprintf(`{ emails = [%q], slack_channels = [%q], teams_channels = [] }`, email, slack),
		testAccCustomerNotificationMinimalSettings))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "destinations.emails.0", strings.ToLower(testUser())),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "destinations.slack_channels.#", "1"),
					resource.TestCheckResourceAttr(testAccCustomerNotificationDataSourceAddr, "destinations.slack_channels.0", slack),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccCustomerNotificationDataSourceAddr, tfjsonpath.New("destinations").AtMapKey("teams_channels"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			testAccCustomerNotificationDriftCheck(config),
		},
	})
}

func TestAccCustomerNotificationDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: `
data "doit_customer_notification" "missing" {
  id = "nonexistent-notification-id"
}
`,
			ExpectError: regexp.MustCompile(`(?i)error reading customer notification|404`),
		}},
	})
}

func TestAccCustomerNotificationsDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-dss")
	config := testAccCustomerNotificationConfig(rName, rName, testAccCustomerNotificationMinimalSettings) + `
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
				Check: testAccCheckCustomerNotificationsItem(rName, map[string]string{
					"settings.cost_anomalies.min_severity":             "warning",
					"settings.cost_anomalies.min_excess_cost.amount":   "1000000.00",
					"settings.cost_anomalies.min_excess_cost.currency": "USD",
					"settings.cost_anomalies.allocations.mode":         "all",
					"settings.cost_anomalies.allocations.ids.#":        "0",
					"destinations.emails.#":                            "1",
					"destinations.slack_channels.#":                    "0",
					"destinations.teams_channels.#":                    "0",
				}),
			},
			testAccCustomerNotificationDriftCheck(config),
		},
	})
}

// testAccCheckCustomerNotificationsItem finds the item of
// doit_customer_notifications.test with the given name and compares its
// attributes, relative to the item.
func testAccCheckCustomerNotificationsItem(name string, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[testAccCustomerNotificationsDataSourceAddr]
		if !ok {
			return fmt.Errorf("%s not found in state", testAccCustomerNotificationsDataSourceAddr)
		}
		attrs := rs.Primary.Attributes
		for key, value := range attrs {
			if !strings.HasPrefix(key, "items.") || !strings.HasSuffix(key, ".name") || value != name {
				continue
			}
			prefix := strings.TrimSuffix(key, "name")
			for attr, expected := range want {
				if got := attrs[prefix+attr]; got != expected {
					return fmt.Errorf("%s%s = %q, want %q", prefix, attr, got, expected)
				}
			}
			if attrs[prefix+"id"] == "" || attrs[prefix+"etag"] == "" || attrs[prefix+"create_time"] == "" {
				return fmt.Errorf("%s: id, etag or create_time is empty", prefix)
			}
			return nil
		}
		return fmt.Errorf("no item named %q in %s", name, testAccCustomerNotificationsDataSourceAddr)
	}
}

// The data source output is valid resource input: a copy built from the
// settings and destinations of the data source plans empty.
func TestAccCustomerNotificationDataSource_Copy(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-cn-ds-copy")
	config := testAccCustomerNotificationConfig(rName, rName, `
      min_severity    = "critical"
      min_excess_cost = { amount = "1000000", currency = "USD" }
      allocations     = { mode = "include", ids = [doit_allocation.a.id] }
      repeat = {
        every_hours = 12
      }
`) + fmt.Sprintf(`
data "doit_customer_notification" "source" {
  id = doit_customer_notification.this.id
}

resource "doit_customer_notification" "copy" {
  name         = "%[1]s-copy"
  destinations = data.doit_customer_notification.source.destinations
  settings     = data.doit_customer_notification.source.settings
}
`, rName)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		CheckDestroy:             testAccCheckCustomerNotificationDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationCopyAddr, "settings.cost_anomalies.min_severity", testAccCustomerNotificationAddr, "settings.cost_anomalies.min_severity"),
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationCopyAddr, "settings.cost_anomalies.repeat.every_hours", testAccCustomerNotificationAddr, "settings.cost_anomalies.repeat.every_hours"),
					resource.TestCheckResourceAttrPair(testAccCustomerNotificationCopyAddr, "destinations.emails.#", testAccCustomerNotificationAddr, "destinations.emails.#"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
