package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// The API enumerates the whole Slack workspace on the first page of every read
// and is rate limited (503 + Retry-After: 30), and the CI tenant uses a large
// production workspace. Each Terraform read of this data source therefore costs
// a full enumeration, and a handful of acceptance tests that each read it a few
// times keep CI waiting on the rate limit for many minutes. Pagination, filter
// forwarding, channels_complete and the response mapping are covered by the unit
// tests in alert_slack_channels_data_source_internal_test.go, so a single live
// request below checks the real API contract end to end.

// TestAccAlertSlackChannelsDataSource_SinglePage makes one page request against
// the real API. It is deliberately not parallel, has a single step and has no
// empty-plan drift step: drift would add more full enumerations, and channels
// can legitimately appear or disappear (and channels_complete can flip) between
// reads of a live workspace.
func TestAccAlertSlackChannelsDataSource_SinglePage(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares the Slack workspace rate limit with other Slack tests
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
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alert_slack_channels.page", "items.#", "5"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.page", "row_count"),
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

// TestAccAlertSlackChannelsDataSource_Basic walks the whole workspace with
// auto-pagination, which takes minutes on the CI tenant. It is skipped by
// default; auto-pagination is covered by the unit tests. Remove the skip to run
// it by hand against a smaller workspace.
func TestAccAlertSlackChannelsDataSource_Basic(t *testing.T) {
	t.Skip("skipped: a full walk of the CI tenant's large Slack workspace is too slow and rate limited for CI; covered by unit tests")

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
