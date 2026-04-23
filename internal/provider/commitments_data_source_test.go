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

// TestAccCommitmentsDataSource_NewAttributes verifies the new commitment-level and
// period-level attributes (total_forecast_value, total_marketplace_spend,
// forecast_value, marketplace_limit_amount, marketplace_spend) are present and
// accessible in the list items.
func TestAccCommitmentsDataSource_NewAttributes(t *testing.T) {
	commitmentCount := getCommitmentCount(t)
	if commitmentCount < 1 {
		t.Skip("Need at least 1 commitment to test new attributes")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentsDataSourceNewAttributesConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_commitments.all", "commitments.#"),
					// Verify new top-level attributes are accessible via output
					resource.TestCheckOutput("has_commitments", "true"),
				),
			},
			// Drift verification
			{
				Config: testAccCommitmentsDataSourceNewAttributesConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
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

func testAccCommitmentsDataSourceNewAttributesConfig() string {
	return `
data "doit_commitments" "all" {
}

output "has_commitments" {
  value = length(data.doit_commitments.all.commitments) > 0
}

# Exercise new commitment-level attributes
output "first_commitment_total_forecast_value" {
  value = length(data.doit_commitments.all.commitments) > 0 ? data.doit_commitments.all.commitments[0].total_forecast_value : null
}

output "first_commitment_total_marketplace_spend" {
  value = length(data.doit_commitments.all.commitments) > 0 ? data.doit_commitments.all.commitments[0].total_marketplace_spend : null
}

# Exercise new period-level attributes
output "first_period_forecast_value" {
  value = (
    length(data.doit_commitments.all.commitments) > 0 &&
    length(data.doit_commitments.all.commitments[0].periods) > 0
  ) ? data.doit_commitments.all.commitments[0].periods[0].forecast_value : null
}

output "first_period_marketplace_spend" {
  value = (
    length(data.doit_commitments.all.commitments) > 0 &&
    length(data.doit_commitments.all.commitments[0].periods) > 0
  ) ? data.doit_commitments.all.commitments[0].periods[0].marketplace_spend : null
}

output "first_period_marketplace_limit_amount" {
  value = (
    length(data.doit_commitments.all.commitments) > 0 &&
    length(data.doit_commitments.all.commitments[0].periods) > 0
  ) ? data.doit_commitments.all.commitments[0].periods[0].marketplace_limit_amount : null
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
