package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
