package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
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
// G Suite asset after import. It fetches the current quantity from the API,
// increments it by one, then restores the original value so subsequent test
// runs aren't no-ops.
func TestAccAssetResource_UpdateQuantity(t *testing.T) {
	assetID := os.Getenv("TEST_ASSET_ID")
	if assetID == "" {
		t.Skip("TEST_ASSET_ID environment variable not set")
	}

	// Fetch current quantity from API so we don't hardcode values.
	currentQty := testAccGetAssetQuantity(t, assetID)
	updatedQty := currentQty + 1

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
			// Update quantity to current + 1
			{
				Config: testAccAssetResourceConfig(assetID, fmt.Sprintf("quantity = %d", updatedQty)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_asset.test", "id", assetID),
					resource.TestCheckResourceAttr("doit_asset.test", "quantity", strconv.Itoa(updatedQty)),
				),
			},
			// Drift verification after update
			{
				Config: testAccAssetResourceConfig(assetID, fmt.Sprintf("quantity = %d", updatedQty)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Restore original quantity so subsequent test runs aren't no-ops
			{
				Config: testAccAssetResourceConfig(assetID, fmt.Sprintf("quantity = %d", currentQty)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_asset.test", "quantity", strconv.Itoa(currentQty)),
				),
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

// testAccGetAssetQuantity fetches the current quantity of an asset from the API.
func testAccGetAssetQuantity(t *testing.T, assetID string) int {
	t.Helper()

	client := getAPIClient(t)
	resp, err := client.GetAssetWithResponse(context.Background(), assetID)
	if err != nil {
		t.Fatalf("Failed to get asset %s: %v", assetID, err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("Failed to get asset %s: status %d, body: %s", assetID, resp.StatusCode(), string(resp.Body))
	}
	if resp.JSON200.Quantity == nil {
		t.Fatalf("Asset %s has no quantity field", assetID)
	}
	return int(*resp.JSON200.Quantity)
}
