package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPlatformsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPlatformsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_platforms.test", "platforms.#"),
					// Verify field values are populated
					resource.TestCheckResourceAttrSet("data.doit_platforms.test", "platforms.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_platforms.test", "platforms.0.display_name"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccPlatformsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccPlatformsDataSourceConfig() string {
	return `
data "doit_platforms" "test" {}
`
}
