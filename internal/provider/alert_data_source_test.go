package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAlertDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "id",
						"doit_alert.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "name",
						"doit_alert.test", "name"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.time_interval",
						"doit_alert.test", "config.time_interval"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.value",
						"doit_alert.test", "config.value"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.operator",
						"doit_alert.test", "config.operator"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAlertDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name = %q
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, name)
}

func TestAccAlertDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAlertDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading alert|not found|404`),
			},
		},
	})
}

func testAccAlertDataSourceNotFoundConfig() string {
	return `
data "doit_alert" "notfound" {
  id = "nonexistent-alert-id"
}
`
}

// TestAccAlertDataSource_WithScopes tests reading an alert with scopes configured
// to exercise the mapScopeToModel function path.
func TestAccAlertDataSource_WithScopes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert-ds-scopes")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceWithScopesConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "id",
						"doit_alert.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.scopes.#",
						"doit_alert.test", "config.scopes.#"),
					resource.TestCheckResourceAttr(
						"data.doit_alert.test", "config.scopes.0.type", "fixed"),
					resource.TestCheckResourceAttr(
						"data.doit_alert.test", "config.scopes.0.id", "cloud_provider"),
				),
			},
		},
	})
}

func testAccAlertDataSourceWithScopesConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name = %q
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "cloud_provider"
        mode   = "is"
        values = ["amazon-web-services"]
      }
    ]
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, name)
}

func TestAccAlertDataSource_IgnoreValuesRange(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert-ds-ivr")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceIgnoreValuesRangeConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "id",
						"doit_alert.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.condition",
						"doit_alert.test", "config.condition"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.operator",
						"doit_alert.test", "config.operator"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.ignore_values_range.lower_bound",
						"doit_alert.test", "config.ignore_values_range.lower_bound"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.ignore_values_range.upper_bound",
						"doit_alert.test", "config.ignore_values_range.upper_bound"),
				),
			},
			// Drift verification
			{
				Config: testAccAlertDataSourceIgnoreValuesRangeConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAlertDataSourceIgnoreValuesRangeConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name = %q
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 50
    currency      = "USD"
    condition     = "percentage-change"
    operator      = "gt"
    ignore_values_range = {
      lower_bound = -15
      upper_bound = 15
    }
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, name)
}

func TestAccAlertDataSource_SlackChannels(t *testing.T) {
	if testSlackChannel() == "" {
		t.Skip("TEST_SLACK_CHAN is not set, skipping Slack channel test")
	}
	rName := acctest.RandomWithPrefix("tf-acc-alert-ds-slack")

	steps := []resource.TestStep{
		{
			Config: testAccAlertDataSourceSlackConfig(rName, testUser(), testSlackChannel()),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair(
					"data.doit_alert.test", "id",
					"doit_alert.test", "id"),
				resource.TestCheckResourceAttrPair(
					"data.doit_alert.test", "recipients_slack_channels.0.id",
					"doit_alert.test", "recipients_slack_channels.0.id"),
				resource.TestCheckResourceAttrPair(
					"data.doit_alert.test", "recipients_slack_channels.0.shared",
					"doit_alert.test", "recipients_slack_channels.0.shared"),
				resource.TestCheckResourceAttrPair(
					"data.doit_alert.test", "recipients_slack_channels.0.workspace",
					"doit_alert.test", "recipients_slack_channels.0.workspace"),
				resource.TestCheckResourceAttr("data.doit_alert.test", "recipients_slack_channels.0.shared", "true"),
				resource.TestCheckNoResourceAttr("data.doit_alert.test", "recipients_slack_channels.0.workspace"),
			),
		},
		// Drift verification
		{
			Config: testAccAlertDataSourceSlackConfig(rName, testUser(), testSlackChannel()),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		},
	}

	if testSlackWorkspaceChannel() != "" && testSlackWorkspace() != "" {
		steps = append([]resource.TestStep{
			{
				Config: testAccAlertDataSourceWorkspaceSlackConfig(rName, testUser(), testSlackWorkspaceChannel(), testSlackWorkspace()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "id",
						"doit_alert.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "recipients_slack_channels.0.id",
						"doit_alert.test", "recipients_slack_channels.0.id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "recipients_slack_channels.0.shared",
						"doit_alert.test", "recipients_slack_channels.0.shared"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "recipients_slack_channels.0.workspace",
						"doit_alert.test", "recipients_slack_channels.0.workspace"),
					resource.TestCheckResourceAttr("data.doit_alert.test", "recipients_slack_channels.0.shared", "false"),
					resource.TestCheckResourceAttr("data.doit_alert.test", "recipients_slack_channels.0.workspace", testSlackWorkspace()),
				),
			},
		}, steps...)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps:                    steps,
	})
}

func testAccAlertDataSourceWorkspaceSlackConfig(name, email, channelID, workspace string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name       = %q
  recipients = [%q]
  recipients_slack_channels = [
    {
      id        = %q
      workspace = %q
    }
  ]
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, name, email, channelID, workspace)
}

func testAccAlertDataSourceSlackConfig(name, email, channelID string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name       = %q
  recipients = [%q]
  recipients_slack_channels = [
    {
      id     = %q
      shared = true
    }
  ]
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, name, email, channelID)
}
