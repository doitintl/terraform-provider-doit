package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccRbacCountriesDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccRbacCountriesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_rbac_countries.test", "countries.#"),
					resource.TestCheckResourceAttrSet("data.doit_rbac_countries.test", "countries.0.country_code"),
					resource.TestCheckResourceAttrSet("data.doit_rbac_countries.test", "countries.0.name"),
				),
			},
			{
				Config: testAccRbacCountriesDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccRbacCountriesDataSourceConfig() string {
	return `
data "doit_rbac_countries" "test" {}
`
}
