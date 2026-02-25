package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccOrganizationsDataSource(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccOrganizationsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify that some data is returned
					resource.TestCheckResourceAttrSet("data.doit_organizations.test", "organizations.#"),
					resource.TestCheckResourceAttrSet("data.doit_organizations.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_organizations.test", "organizations.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_organizations.test", "organizations.0.name"),
				),
			},
			// Drift verification
			{
				Config: testAccOrganizationsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccOrganizationsDataSourceConfig() string {
	return `
data "doit_organizations" "test" {}
`
}
