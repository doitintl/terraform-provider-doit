package provider_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	testAccUserNotificationDataSourceAddr  = "data.doit_user_notification.test"
	testAccUserNotificationsDataSourceAddr = "data.doit_user_notifications.test"
	testAccUserNotificationCopyAddr        = "doit_user_notification.copy"
)

// TEST_USER is shared with every other user notification test running in
// parallel, so the list tests find their notification by ID or name and never
// assert a count or an index. The copy tests are the only writers on TEST_USER_2.

func testAccUserNotificationDataSourceConfig(rName, name, costAnomalies string) string {
	return testAccUserNotificationConfig(rName, name, costAnomalies) + `
data "doit_user_notification" "test" {
  user_id = doit_user_notification.this.user_id
  id      = doit_user_notification.this.id
}
`
}

func TestAccUserNotificationDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-ds")
	config := testAccUserNotificationDataSourceConfig(rName, rName, testAccUserNotificationMinimalSettings)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "id", testAccUserNotificationAddr, "id"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "user_id", testAccUserNotificationAddr, "user_id"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "name", testAccUserNotificationAddr, "name"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "etag", testAccUserNotificationAddr, "etag"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "create_time", testAccUserNotificationAddr, "create_time"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "update_time", testAccUserNotificationAddr, "update_time"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "destinations.#", testAccUserNotificationAddr, "destinations.#"),
					resource.TestCheckResourceAttrPair(testAccUserNotificationDataSourceAddr, "destinations.0.address", testAccUserNotificationAddr, "destinations.0.address"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "destinations.0.type", "email"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.min_severity", "warning"),
					// The data source reports the API's spelling of the amount, not the configured "1000000".
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.min_excess_cost.amount", "1000000.00"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.min_excess_cost.currency", "USD"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.allocations.mode", "all"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.allocations.ids.#", "0"),
					resource.TestCheckNoResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.every_hours"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccUserNotificationDataSourceAddr,
						testAccUserNotificationPath("repeat"), knownvalue.Null()),
				},
			},
			testAccUserNotificationDriftCheck(config),
		},
	})
}

func TestAccUserNotificationDataSource_Repeat(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-ds-rep")
	config := testAccUserNotificationDataSourceConfig(rName, rName, `
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
`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.min_severity", "critical"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.allocations.mode", "include"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.allocations.ids.#", "2"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.every_hours", "12"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.match", "and"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.percent_increase", "10"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.within_hours", "6"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.min_increase.amount", "150.00"),
					resource.TestCheckResourceAttr(testAccUserNotificationDataSourceAddr, "settings.cost_anomalies.repeat.when_cost_grows.min_increase.currency", "USD"),
				),
			},
			testAccUserNotificationDriftCheck(config),
		},
	})
}

func TestAccUserNotificationDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
data "doit_users" "test" {
  email = %q
}

data "doit_user_notification" "missing" {
  user_id = data.doit_users.test.users[0].id
  id      = "nonexistent-notification-id"
}
`, testUser()),
			ExpectError: regexp.MustCompile(`(?i)error reading user notification|404`),
		}},
	})
}

func TestAccUserNotificationsDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-un-dss")
	config := testAccUserNotificationConfig(rName, rName, testAccUserNotificationMinimalSettings) + `
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
					resource.TestCheckResourceAttrPair(testAccUserNotificationsDataSourceAddr, "user_id", testAccUserNotificationAddr, "user_id"),
					testAccCheckUserNotificationsItem(rName, map[string]string{
						"settings.cost_anomalies.min_severity":             "warning",
						"settings.cost_anomalies.min_excess_cost.amount":   "1000000.00",
						"settings.cost_anomalies.min_excess_cost.currency": "USD",
						"settings.cost_anomalies.allocations.mode":         "all",
						"settings.cost_anomalies.allocations.ids.#":        "0",
						"destinations.#":      "1",
						"destinations.0.type": "email",
					}),
				),
			},
			testAccUserNotificationDriftCheck(config),
		},
	})
}

// testAccCheckUserNotificationsItem finds the item of doit_user_notifications.test
// with the given name and compares its attributes, relative to the item.
func testAccCheckUserNotificationsItem(name string, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[testAccUserNotificationsDataSourceAddr]
		if !ok {
			return fmt.Errorf("%s not found in state", testAccUserNotificationsDataSourceAddr)
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
		return fmt.Errorf("no item named %q in %s", name, testAccUserNotificationsDataSourceAddr)
	}
}

// testAccUserNotificationCopyConfig reads the notification named rName from
// TEST_USER with the list data source and creates a copy of it on TEST_USER_2.
func testAccUserNotificationCopyConfig(rName, costAnomalies string) string {
	return testAccUserNotificationConfig(rName, rName, costAnomalies) + fmt.Sprintf(`
data "doit_user_notifications" "source" {
  user_id    = doit_user_notification.this.user_id
  depends_on = [doit_user_notification.this]
}

data "doit_users" "target" {
  email = %[1]q
}

locals {
  source = one([for n in data.doit_user_notifications.source.items : n if n.name == %[2]q])
}

resource "doit_user_notification" "copy" {
  user_id  = data.doit_users.target.users[0].id
  name     = "%[2]s-copy"
  settings = local.source.settings
}
`, testUser2(), rName)
}

func TestAccUserNotificationDataSource_CopyToAnotherUser(t *testing.T) {
	if testUser2() == "" {
		t.Skip("TEST_USER_2 must be set")
	}
	rName := acctest.RandomWithPrefix("tf-acc-un-copy")
	config := testAccUserNotificationCopyConfig(rName, testAccUserNotificationMinimalSettings)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "name", rName+"-copy"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.min_severity", "warning"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.min_excess_cost.amount", "1000000.00"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.allocations.mode", "all"),
					resource.TestCheckNoResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.repeat.every_hours"),
				),
			},
			// The copy must not drift: the API's spelling of the amount and the
			// absent repeat survive the data source → resource round trip.
			testAccUserNotificationDriftCheck(config),
		},
	})
}

func TestAccUserNotificationDataSource_CopyToAnotherUserWithRepeat(t *testing.T) {
	if testUser2() == "" {
		t.Skip("TEST_USER_2 must be set")
	}
	rName := acctest.RandomWithPrefix("tf-acc-un-copy-rep")
	config := testAccUserNotificationCopyConfig(rName, `
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
`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.min_severity", "critical"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.allocations.ids.#", "2"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.repeat.every_hours", "12"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.repeat.when_cost_grows.match", "and"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.repeat.when_cost_grows.percent_increase", "10"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.repeat.when_cost_grows.within_hours", "6"),
					resource.TestCheckResourceAttr(testAccUserNotificationCopyAddr, "settings.cost_anomalies.repeat.when_cost_grows.min_increase.amount", "150.00"),
				),
			},
			testAccUserNotificationDriftCheck(config),
		},
	})
}
