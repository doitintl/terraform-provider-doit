package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccAssetsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccAssetsDataSource_MaxResultsOnly(t *testing.T) {
	assetCount := getAssetCount(t)
	if assetCount < 3 {
		t.Skipf("Need at least 3 assets to test pagination, got %d", assetCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetsDataSourceMaxResultsConfig(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_assets.limited", "assets.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_assets.limited", "page_token"),
				),
			},
			{
				Config:   testAccAssetsDataSourceMaxResultsConfig(2),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAssetsDataSourceMaxResultsConfig(2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAssetsDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_assets" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccAssetsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning results.
// Uses chained data sources to avoid page token expiry.
func TestAccAssetsDataSource_PageTokenOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetsDataSourceChainedPageTokenOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_assets.from_token", "assets.#"),
					resource.TestCheckResourceAttrSet("data.doit_assets.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_assets.from_token", "page_token"),
				),
			},
		},
	})
}

func testAccAssetsDataSourceChainedPageTokenOnly() string {
	return `
data "doit_assets" "first_page" {
  max_results = 1
}
data "doit_assets" "from_token" {
  page_token = data.doit_assets.first_page.page_token
}
`
}

// TestAccAssetsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses chained data sources to avoid page token expiry.
func TestAccAssetsDataSource_MaxResultsAndPageToken(t *testing.T) {
	assetsCount := getAssetCount(t)
	if assetsCount < 3 {
		t.Skipf("Need at least 3 assets to test pagination, got %d", assetsCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetsDataSourceChainedMaxResultsAndPageToken(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_assets.second_page", "assets.#", "1"),
				),
			},
		},
	})
}

func testAccAssetsDataSourceChainedMaxResultsAndPageToken() string {
	return `
data "doit_assets" "first_page" {
  max_results = 1
}
data "doit_assets" "second_page" {
  max_results = 1
  page_token  = data.doit_assets.first_page.page_token
}
`
}

// TestAccAssetsDataSource_AutoPagination tests that without max_results, all assets are fetched.
func TestAccAssetsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_assets.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_assets.test", "page_token"),
				),
			},
		},
	})
}

func testAccAssetsDataSourceConfig() string {
	return `
data "doit_assets" "test" {
}
`
}

// Helper functions

var (
	assetCount     int
	assetCountOnce sync.Once
)

func getAssetCount(t *testing.T) int {
	t.Helper()
	assetCountOnce.Do(func() {
		assetCount = computeAssetCount(t)
	})
	return assetCount
}

func computeAssetCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.IdOfAssetsParams{}
	for {
		resp, err := client.IdOfAssetsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list assets: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Assets == nil {
			break
		}
		total += len(*resp.JSON200.Assets)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
