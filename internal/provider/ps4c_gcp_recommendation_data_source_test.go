package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpRecommendationDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpRecommendationDataSourceConfig("compute", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendation.test", "billing_account_id"),
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_recommendation.test", "gcp_service", "compute"),
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_recommendation.test", "granularity", "day"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendation.test", "estimated_equivalent_recommended_commitment"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpRecommendationDataSourceConfig("compute", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpRecommendationDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccPs4cGcpRecommendationDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)not found|status 404`),
			},
		},
	})
}

func testAccPs4cGcpRecommendationDataSourceConfig(service, region string) string {
	regionConfig := ""
	if region != "" {
		regionConfig = `
  region = "` + region + `"`
	}

	return `
data "doit_ps4c_gcp_billing_accounts" "list" {}

locals {
  eligible_accounts = [
    for account in data.doit_ps4c_gcp_billing_accounts.list.items :
    account if anytrue(concat(
      [for item in try(account.onboarding_status, []) : item.service == "` + service + `"],
      [for item in try(account.stats30d, []) : item.service == "` + service + `"],
      [for item in try(account.savings_totals, []) : item.service == "` + service + `"]
    ))
  ]
  selected_billing_account_id = length(local.eligible_accounts) > 0 ? local.eligible_accounts[0].billing_account_id : "000000-000000-000000"
}

data "doit_ps4c_gcp_recommendation" "test" {
  billing_account_id = local.selected_billing_account_id
  gcp_service        = "` + service + `"` + regionConfig + `

  lifecycle {
    precondition {
      condition     = length(local.eligible_accounts) > 0
      error_message = "No PS4C GCP billing accounts with '` + service + `' onboarding/stats found; cannot run doit_ps4c_gcp_recommendation tests."
    }
  }
}
`
}

func testAccPs4cGcpRecommendationDataSourceNotFoundConfig() string {
	return `
data "doit_ps4c_gcp_recommendation" "test" {
  billing_account_id = "000000-000000-000000"
  gcp_service        = "compute"
}
`
}
