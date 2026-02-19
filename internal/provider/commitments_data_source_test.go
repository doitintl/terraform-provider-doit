package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
func TestAccCommitmentsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getCommitmentFirstPageToken(t, "1")
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 commitment)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_commitments.from_token", "commitments.#"),
				),
			},
		},
	})
}

func testAccCommitmentsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_commitments" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccCommitmentsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccCommitmentsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getCommitmentFirstPageToken(t, "1")
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 commitment)")
	}

	commitmentCount := getCommitmentCount(t)
	if commitmentCount < 3 {
		t.Skipf("Need at least 3 commitments to test pagination, got %d", commitmentCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_commitments.paginated", "commitments.#", "1"),
				),
			},
		},
	})
}

func testAccCommitmentsDataSourceMaxResultsAndPageTokenConfig(maxResults string, pageToken string) string {
	return fmt.Sprintf(`
data "doit_commitments" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
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

func getCommitmentFirstPageToken(t *testing.T, maxResults string) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	resp, err := client.ListCommitmentsWithResponse(ctx, &models.ListCommitmentsParams{
		MaxResults: &maxResults,
	})
	if err != nil {
		t.Fatalf("Failed to list commitments: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
