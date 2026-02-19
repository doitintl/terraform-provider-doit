package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccProductsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccProductsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_products.test", "products.#"),
					// Verify field values are populated
					resource.TestCheckResourceAttrSet("data.doit_products.test", "products.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_products.test", "products.0.display_name"),
					resource.TestCheckResourceAttrSet("data.doit_products.test", "products.0.platform"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccProductsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccProductsDataSourceConfig() string {
	return `
data "doit_products" "test" {}
`
}

func TestAccProductsDataSource_WithPlatformFilter(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccProductsDataSourceWithPlatformConfig("google_cloud_platform"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_products.filtered", "products.#"),
					resource.TestCheckResourceAttr("data.doit_products.filtered", "platform", "google_cloud_platform"),
				),
			},
			// Drift verification: re-apply with filter should produce an empty plan
			{
				Config: testAccProductsDataSourceWithPlatformConfig("google_cloud_platform"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccProductsDataSourceWithPlatformConfig(platform string) string {
	return `
data "doit_products" "filtered" {
  platform = "` + platform + `"
}
`
}
