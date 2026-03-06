package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccAssetResource_Import tests importing an existing G Suite asset and
// verifying all computed fields are populated, and that subsequent plans are empty.
func TestAccAssetResource_Import(t *testing.T) {
	assetID := os.Getenv("TEST_ASSET_ID")
	if assetID == "" {
		t.Skip("TEST_ASSET_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Import the existing asset and persist state
			{
				Config:             testAccAssetResourceConfig(assetID, ""),
				ResourceName:       "doit_asset.test",
				ImportState:        true,
				ImportStateId:      assetID,
				ImportStatePersist: true,
				ImportStateVerify:  false,
			},
			// Step 2: Verify fields are populated after import
			{
				Config: testAccAssetResourceConfig(assetID, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_asset.test", "id", assetID),
					resource.TestCheckResourceAttrSet("doit_asset.test", "name"),
					resource.TestCheckResourceAttr("doit_asset.test", "type", "g-suite"),
					resource.TestCheckResourceAttrSet("doit_asset.test", "url"),
					resource.TestCheckResourceAttrSet("doit_asset.test", "quantity"),
				),
			},
			// Step 3: Drift verification - re-apply should produce an empty plan
			{
				Config: testAccAssetResourceConfig(assetID, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccAssetResource_ImportAWS tests importing an AWS asset, which does not
// have a quantity field.
func TestAccAssetResource_ImportAWS(t *testing.T) {
	assetID := os.Getenv("TEST_ASSET_ID_AWS")
	if assetID == "" {
		t.Skip("TEST_ASSET_ID_AWS environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Import the existing AWS asset
			{
				Config:             testAccAssetResourceConfig(assetID, ""),
				ResourceName:       "doit_asset.test",
				ImportState:        true,
				ImportStateId:      assetID,
				ImportStatePersist: true,
				ImportStateVerify:  false,
			},
			// Verify the asset type is AWS
			{
				Config: testAccAssetResourceConfig(assetID, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_asset.test", "id", assetID),
					resource.TestCheckResourceAttr("doit_asset.test", "type", "amazon-web-services"),
				),
			},
		},
	})
}

// TestAccAssetResource_CreateErrors tests that creating an asset without
// importing produces a clear error message.
func TestAccAssetResource_CreateErrors(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAssetResourceConfig("some-nonexistent-asset", ""),
				ExpectError: regexp.MustCompile(`Asset Creation Not Supported`),
			},
		},
	})
}

// TestAccAssetResource_UpdateQuantity tests updating the quantity field on a
// G Suite asset after import.
func TestAccAssetResource_UpdateQuantity(t *testing.T) {
	assetID := os.Getenv("TEST_ASSET_ID")
	if assetID == "" {
		t.Skip("TEST_ASSET_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Import first (persist state for subsequent steps)
			{
				Config:             testAccAssetResourceConfig(assetID, ""),
				ResourceName:       "doit_asset.test",
				ImportState:        true,
				ImportStateId:      assetID,
				ImportStatePersist: true,
				ImportStateVerify:  false,
			},
			// Update quantity
			{
				Config: testAccAssetResourceConfig(assetID, "quantity = 3"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_asset.test", "id", assetID),
					resource.TestCheckResourceAttr("doit_asset.test", "quantity", "3"),
				),
			},
			// Drift verification after update
			{
				Config: testAccAssetResourceConfig(assetID, "quantity = 3"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAssetResourceConfig(id string, extraAttrs string) string {
	if extraAttrs != "" {
		return fmt.Sprintf(`
resource "doit_asset" "test" {
  id = %[1]q
  %[2]s
}
`, id, extraAttrs)
	}
	return fmt.Sprintf(`
resource "doit_asset" "test" {
  id = %[1]q
}
`, id)
}
