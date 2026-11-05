package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccBudget_CollaboratorWithoutRoleRejected checks that a collaborator
// without a role fails at plan time. The schema marks role Optional+Computed,
// but the API answers a missing role with 400 "invalid role" and assigns no
// default, so before the validator this only failed at apply.
func TestAccBudget_CollaboratorWithoutRoleRejected(t *testing.T) {
	n := acctest.RandInt()
	config := fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-omitted-role-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  scopes = [{
    type   = "allocation_rule"
    id     = "allocation_rule"
    mode   = "is"
    values = ["%s"]
  }]
  alerts = [{ percentage = 50 }]
  collaborators = [
    { email = "%s", role = "owner" },
    { email = "other@example.com" },
  ]
}
`, budgetStartPeriod(), n, testAttribution(), testUser())

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {Source: "hashicorp/time", VersionConstraint: "~> 0.13"},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      config,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Collaborator Role Required`),
		}},
	})
}

// TestAccBudget_SlackChannelSurvivesUnrelatedUpdate updates an unrelated
// attribute of a budget that keeps a Slack channel configured with only
// id, customer_id and workspace. The channel's name, shared and type are
// Optional+Computed without a Default, so they plan unknown on that update and
// must not reach the request as empty values.
func TestAccBudget_SlackChannelSurvivesUnrelatedUpdate(t *testing.T) {
	if testSlackChannel() == "" || testSlackWorkspace() == "" || testCustomerID() == "" {
		t.Skip("TEST_SLACK_CHAN, TEST_SLACK_WORKSPACE or TEST_CUSTOMER_ID is not set, skipping Slack channel test")
	}
	n := acctest.RandInt()
	config := func(amount int) string {
		return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-slack-update-%d"
  currency      = "EUR"
  time_interval = "quarter"
  type          = "recurring"
  start_period  = local.start_period
  amount        = %d
  recipients_slack_channels = [{
    id          = "%s"
    customer_id = "%s"
    workspace   = "%s"
  }]
  scopes = [{
    type   = "allocation_rule"
    id     = "allocation_rule"
    mode   = "is"
    values = ["%s"]
  }]
}
`, budgetStartPeriod(), n, amount, testSlackChannel(), testCustomerID(), testSlackWorkspace(), testAttribution())
	}
	channelPath := tfjsonpath.New("recipients_slack_channels").AtSliceIndex(0)

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {Source: "hashicorp/time", VersionConstraint: "~> 0.13"},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config(100)},
			{
				Config: config(200),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectUnknownValue("doit_budget.this", channelPath.AtMapKey("name")),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_budget.this", channelPath.AtMapKey("id"), knownvalue.StringExact(testSlackChannel())),
				},
			},
			{Config: config(200), PlanOnly: true},
		},
	})
}
