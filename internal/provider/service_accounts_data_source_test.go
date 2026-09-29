package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccServiceAccountsDataSource_Basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-sas-ds")
	config := testAccServiceAccountsDataSourceConfig(name)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("list_contains_account", "true"),
					resource.TestCheckOutput("list_matches_account", "true"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccServiceAccountsDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_service_account" "test" {
  name        = %q
  description = "List data source acceptance test"
  permissions = ["budgetsReadOnly"]
}

data "doit_service_accounts" "all" {
  depends_on = [doit_service_account.test]
}

locals {
  matching_accounts = [for account in data.doit_service_accounts.all.items : account if account.id == doit_service_account.test.id]
}

output "list_contains_account" {
  value = length(local.matching_accounts) == 1
}

output "list_matches_account" {
  value = alltrue([for account in local.matching_accounts :
    account.name == doit_service_account.test.name &&
    account.description == doit_service_account.test.description &&
    account.permissions == doit_service_account.test.permissions &&
    account.customer_id != null && account.create_time != null && account.etag != null
  ])
}
`, name)
}
