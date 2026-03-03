package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAssetDataSource_Basic(t *testing.T) {
	assetID := os.Getenv("TEST_ASSET_ID")
	if assetID == "" {
		t.Skip("TEST_ASSET_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetDataSourceConfig(assetID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_asset.test", "id", assetID),
					resource.TestCheckResourceAttrSet("data.doit_asset.test", "type"),
					resource.TestCheckResourceAttrSet("data.doit_asset.test", "quantity"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAssetDataSourceConfig(assetID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAssetDataSource_AWS tests retrieval of an AWS asset, which notably
// omits the quantity field (unlike G Suite assets).
func TestAccAssetDataSource_AWS(t *testing.T) {
	assetID := os.Getenv("TEST_ASSET_ID_AWS")
	if assetID == "" {
		t.Skip("TEST_ASSET_ID_AWS environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetDataSourceConfig(assetID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_asset.test", "id", assetID),
					resource.TestCheckResourceAttr("data.doit_asset.test", "type", "amazon-web-services"),
					resource.TestCheckNoResourceAttr("data.doit_asset.test", "quantity"),
				),
			},
			// Drift verification
			{
				Config: testAccAssetDataSourceConfig(assetID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAssetDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_asset" "test" {
  id = %[1]q
}
`, id)
}
