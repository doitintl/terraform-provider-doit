package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpBillingAccountsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpBillingAccountsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_accounts.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_accounts.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_accounts.test", "items.0.billing_account_id"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_accounts.test", "items.0.onboarding_status.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_accounts.test", "items.0.stats30d.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_billing_accounts.test", "items.0.savings_totals.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpBillingAccountsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccPs4cGcpBillingAccountsDataSourceConfig() string {
	return `
data "doit_ps4c_gcp_billing_accounts" "test" {
  lifecycle {
    postcondition {
      condition = (
        alltrue(flatten([for account in self.items : [for item in account.onboarding_status : contains(["compute", "cloud_sql"], item.service)]])) &&
        alltrue(flatten([for account in self.items : [for item in account.stats30d : contains(["compute", "cloud_sql"], item.service)]])) &&
        alltrue(flatten([for account in self.items : [for item in account.savings_totals : contains(["compute", "cloud_sql"], item.service)]]))
      )
      error_message = "PS4C GCP account summaries must use compute or cloud_sql service entries."
    }
  }
}
`
}
