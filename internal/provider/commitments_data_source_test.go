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

// TestAccCommitmentsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccCommitmentsDataSource_MaxResultsOnly(t *testing.T) {
	commitmentCount := getCommitmentCount(t)
	if commitmentCount < 2 {
		t.Skipf("Need at least 2 commitments to test pagination, got %d", commitmentCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourceMaxResultsConfig("1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_commitments.limited", "commitments.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_commitments.limited", "page_token"),
				),
			},
			{
				Config:   testAccCommitmentsDataSourceMaxResultsConfig("1"),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccCommitmentsDataSourceMaxResultsConfig("1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCommitmentsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_commitments" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccCommitmentsDataSource_PageTokenOnly tests using a page_token from a previous API call.
// TestAccCommitmentsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
// Uses chained data sources to avoid page token expiry.
func TestAccCommitmentsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): API ignores pageToken when maxResults is not set.
	t.Skip("Skipping: API ignores pageToken without maxResults (CMP-38591)")
	totalCommitments := getCommitmentCount(t)
	if totalCommitments < 2 {
		t.Skipf("Need at least 2 commitments to test page_token-only, got %d", totalCommitments)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourceChainedPageTokenOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_commitments.from_token", "commitments.#"),
					testCheckResourceAttrLessThan("data.doit_commitments.from_token", "row_count", totalCommitments),
				),
			},
		},
	})
}

func testAccCommitmentsDataSourceChainedPageTokenOnly() string {
	return `
data "doit_commitments" "first_page" {
  max_results = "1"
}
data "doit_commitments" "from_token" {
  page_token = data.doit_commitments.first_page.page_token
}
`
}

// TestAccCommitmentsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses chained data sources to avoid page token expiry.
func TestAccCommitmentsDataSource_MaxResultsAndPageToken(t *testing.T) {
	commitmentsCount := getCommitmentCount(t)
	if commitmentsCount < 3 {
		t.Skipf("Need at least 3 commitments to test pagination, got %d", commitmentsCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourceChainedMaxResultsAndPageToken(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_commitments.second_page", "commitments.#", "1"),
				),
			},
		},
	})
}

func testAccCommitmentsDataSourceChainedMaxResultsAndPageToken() string {
	return `
data "doit_commitments" "first_page" {
  max_results = "1"
}
data "doit_commitments" "second_page" {
  max_results = "1"
  page_token  = data.doit_commitments.first_page.page_token
}
`
}

// TestAccCommitmentsDataSource_AutoPagination tests that without max_results, all commitments are fetched.
func TestAccCommitmentsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_commitments.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_commitments.test", "page_token"),
				),
			},
		},
	})
}

func testAccCommitmentsDataSourceConfig() string {
	return `
data "doit_commitments" "test" {
}
`
}

// Helper functions

var (
	commitmentCount     int
	commitmentCountOnce sync.Once
)

func getCommitmentCount(t *testing.T) int {
	t.Helper()
	commitmentCountOnce.Do(func() {
		commitmentCount = computeCommitmentCount(t)
	})
	return commitmentCount
}

func computeCommitmentCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListCommitmentsParams{}
	for {
		resp, err := client.ListCommitmentsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list commitments: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Commitments == nil {
			break
		}
		total += len(*resp.JSON200.Commitments)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
