package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpBillingAccountDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpBillingAccountDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_account.test", "billing_account_id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_ps4c_gcp_billing_account.test", "billing_account_id",
						"data.doit_ps4c_gcp_billing_accounts.list", "items.0.billing_account_id",
					),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpBillingAccountDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpBillingAccountDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccPs4cGcpBillingAccountDataSourceConfigDirect("000000-000000-000000"),
				ExpectError: regexp.MustCompile(`(?i)not found|status 404`),
			},
		},
	})
}

func testAccPs4cGcpBillingAccountDataSourceConfig() string {
	return `
data "doit_ps4c_gcp_billing_accounts" "list" {}

locals {
  accounts = data.doit_ps4c_gcp_billing_accounts.list.items
  first_billing_account_id = length(local.accounts) > 0 ? local.accounts[0].billing_account_id : "000000-000000-000000"
}

data "doit_ps4c_gcp_billing_account" "test" {
  billing_account_id = local.first_billing_account_id

  lifecycle {
    precondition {
      condition     = length(local.accounts) > 0
      error_message = "No PS4C GCP billing accounts returned; cannot run doit_ps4c_gcp_billing_account tests."
    }
  }
}
`
}

func testAccPs4cGcpBillingAccountDataSourceConfigDirect(billingAccountID string) string {
	return `
data "doit_ps4c_gcp_billing_account" "test" {
  billing_account_id = "` + billingAccountID + `"
}
`
}
