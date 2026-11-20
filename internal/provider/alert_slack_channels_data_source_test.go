package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The API pages through Slack's own cursor and reads at most five 200-channel
// slices per request, so every request is bounded and the slices are cached for
// five minutes. These tests therefore run in parallel like any other data source
// test. The CI tenant uses a large production workspace (thousands of channels),
// which the assertions below rely on: with max_results = 5 there is always a
// next page, so row_count is never returned.
//
// The tests do not assert channels_complete: the API reports it as false when
// the caller is a service account or lacks the private-channel scope, and the
// value can also flip between reads of a live workspace.

// TestAccAlertSlackChannelsDataSource_SinglePage reads one page and checks the
// response mapping. There is no empty-plan drift step because channels can
// legitimately appear or disappear between reads of a live workspace.
func TestAccAlertSlackChannelsDataSource_SinglePage(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_alert_slack_channels" "page" {
  max_results = 5
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					// The API returns rowCount only when one request saw the whole list.
					statecheck.ExpectKnownValue("data.doit_alert_slack_channels.page", tfjsonpath.New("row_count"), knownvalue.Null()),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alert_slack_channels.page", "items.#", "5"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "workspace_status"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "is_workspace_connected"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "has_shared_channel"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "channels_complete"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "items.0.name"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "items.0.type"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "items.0.shared"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "items.0.customer_id"),
				),
			},
		},
	})
}

// TestAccAlertSlackChannelsDataSource_ManualPagingTwoPages feeds the page_token of
// one read into a second read and checks that the second page is a different set
// of channels.
func TestAccAlertSlackChannelsDataSource_ManualPagingTwoPages(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_alert_slack_channels" "first" {
  max_results = 5
}

data "doit_alert_slack_channels" "second" {
  max_results = 5
  page_token  = data.doit_alert_slack_channels.first.page_token
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_alert_slack_channels.second", tfjsonpath.New("items"), knownvalue.ListSizeExact(5)),
					statecheck.CompareValuePairs(
						"data.doit_alert_slack_channels.first", tfjsonpath.New("items").AtSliceIndex(0).AtMapKey("id"),
						"data.doit_alert_slack_channels.second", tfjsonpath.New("items").AtSliceIndex(0).AtMapKey("id"),
						compare.ValuesDiffer(),
					),
					statecheck.CompareValuePairs(
						"data.doit_alert_slack_channels.first", tfjsonpath.New("page_token"),
						"data.doit_alert_slack_channels.second", tfjsonpath.New("page_token"),
						compare.ValuesDiffer(),
					),
				},
			},
		},
	})
}

// TestAccAlertSlackChannelsDataSource_InvalidPageToken checks that a cursor the API
// rejects surfaces as an error with the restart hint instead of an empty result.
// Terraform wraps long error lines, so the pattern allows any whitespace.
func TestAccAlertSlackChannelsDataSource_InvalidPageToken(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_alert_slack_channels" "bogus" {
  max_results = 5
  page_token  = "bogus"
}
`,
				ExpectError: regexp.MustCompile(`invalid pageToken[\s\S]*read\s+again\s+without\s+page_token`),
			},
		},
	})
}

// TestAccAlertSlackChannelsDataSource_Basic auto-paginates the whole workspace with a
// name filter. Pages can be short or empty while the filter discards most of the
// scanned channels, so this covers following page_token to its end. The second step
// re-reads the same list, which the API serves from its five-minute slice cache.
func TestAccAlertSlackChannelsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_alert_slack_channels" "test" {
  name_contains = "budget"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "workspace_status"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "channels_complete"),
					resource.TestCheckNoResourceAttr("data.doit_alert_slack_channels.test", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "items.0.id"),
				),
			},
			{
				Config: `
data "doit_alert_slack_channels" "test" {
  name_contains = "budget"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
