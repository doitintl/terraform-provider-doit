package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccUsersDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_users.test", "users.#"),
					resource.TestCheckResourceAttrSet("data.doit_users.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_users.test", "users.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_users.test", "users.0.email"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccUsersDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccUsersDataSourceConfig() string {
	return `
data "doit_users" "test" {}
`
}
