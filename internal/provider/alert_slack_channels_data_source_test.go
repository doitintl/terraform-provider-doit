package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

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
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "is_workspace_connected"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "has_shared_channel"),
					resource.TestCheckNoResourceAttr("data.doit_alert_slack_channels.test", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "items.0.name"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "items.0.type"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "items.0.shared"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.test", "items.0.customer_id"),
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

func TestAccAlertSlackChannelsDataSource_MaxResultsOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_alert_slack_channels" "limited" {
  max_results = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alert_slack_channels.limited", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.limited", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.limited", "workspace_status"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.limited", "is_workspace_connected"),
					resource.TestCheckResourceAttrSet("data.doit_alert_slack_channels.limited", "has_shared_channel"),
				),
			},
			{
				Config: `
data "doit_alert_slack_channels" "limited" {
  max_results = 1
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

func TestAccAlertSlackChannelsDataSource_NameContains(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "doit_alert_slack_channels" "filtered" {
  name_contains = %q
}
`, "nonexistent-slack-channel-xyz-99999"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alert_slack_channels.filtered", "items.#", "0"),
					resource.TestCheckResourceAttr("data.doit_alert_slack_channels.filtered", "row_count", "0"),
				),
			},
			{
				Config: fmt.Sprintf(`
data "doit_alert_slack_channels" "filtered" {
  name_contains = %q
}
`, "nonexistent-slack-channel-xyz-99999"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
