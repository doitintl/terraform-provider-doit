package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAlert_SlackChannelSurvivesUnrelatedUpdate updates an unrelated
// attribute of an alert that keeps a Slack channel configured by id only. The
// channel's customer_id and workspace are Optional+Computed without a Default, so
// they plan unknown on that update and must not reach the request as empty values.
func TestAccAlert_SlackChannelSurvivesUnrelatedUpdate(t *testing.T) {
	if testSlackChannel() == "" {
		t.Skip("TEST_SLACK_CHAN is not set, skipping Slack channel test")
	}
	n := acctest.RandInt()
	config := func(value int) string {
		return fmt.Sprintf(`
resource "doit_alert" "this" {
  name       = "test-alert-slack-update-%d"
  recipients = ["%s"]
  recipients_slack_channels = [{
    id     = "%s"
    shared = true
  }]
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = %d
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}
`, n, testUser(), testSlackChannel(), value)
	}
	channelPath := tfjsonpath.New("recipients_slack_channels").AtSliceIndex(0)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config(1000)},
			{
				Config: config(2000),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue("doit_alert.this", channelPath.AtMapKey("customer_id")),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_alert.this", channelPath.AtMapKey("id"), knownvalue.StringExact(testSlackChannel())),
					statecheck.ExpectKnownValue("doit_alert.this", channelPath.AtMapKey("shared"), knownvalue.Bool(true)),
				},
			},
			{Config: config(2000), PlanOnly: true},
		},
	})
}
