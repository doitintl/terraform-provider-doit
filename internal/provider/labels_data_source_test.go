package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccLabelsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccLabelsDataSource_MaxResultsOnly(t *testing.T) {
	labelCount := getLabelCount(t)
	if labelCount < 3 {
		t.Skipf("Need at least 3 labels to test pagination, got %d", labelCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_labels.limited", "labels.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_labels.limited", "page_token"),
				),
			},
			{
				Config:   testAccLabelsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
		},
	})
}

func testAccLabelsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_labels" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccLabelsDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccLabelsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getLabelFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 label)")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_labels.from_token", "labels.#"),
				),
			},
		},
	})
}

func testAccLabelsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_labels" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccLabelsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccLabelsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getLabelFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 label)")
	}

	labelCount := getLabelCount(t)
	if labelCount < 3 {
		t.Skipf("Need at least 3 labels to test pagination, got %d", labelCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_labels.paginated", "labels.#", "1"),
				),
			},
		},
	})
}

func testAccLabelsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_labels" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccLabelsDataSource_AutoPagination tests that without max_results, all labels are fetched.
func TestAccLabelsDataSource_AutoPagination(t *testing.T) {
	expectedCount := getLabelCount(t)
	if expectedCount == 0 {
		t.Skip("No labels available to test auto-pagination")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_labels.test", "row_count", fmt.Sprintf("%d", expectedCount)),
					resource.TestCheckNoResourceAttr("data.doit_labels.test", "page_token"),
				),
			},
		},
	})
}

func testAccLabelsDataSourceConfig() string {
	return `
data "doit_labels" "test" {
}
`
}

// Helper functions

func getLabelCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListLabelsParams{}
	for {
		resp, err := client.ListLabelsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list labels: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Labels == nil {
			break
		}
		total += len(*resp.JSON200.Labels)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}

func getLabelFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListLabelsWithResponse(ctx, &models.ListLabelsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list labels: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
