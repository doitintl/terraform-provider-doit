package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCustomerDataSource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton: shares customer state with resource tests
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomerDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "name"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "primary_domain"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "domains.#"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "settings.currency"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "settings.allowed_invite_domains.#"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "contact.emails.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccCustomerDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCustomerDataSourceConfig() string {
	return `
data "doit_customer" "test" {}
`
}
