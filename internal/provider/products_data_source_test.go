package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
				),
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
