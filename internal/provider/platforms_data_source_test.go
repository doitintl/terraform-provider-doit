package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
		},
	})
}

func testAccPlatformsDataSourceConfig() string {
	return `
data "doit_platforms" "test" {}
`
}
