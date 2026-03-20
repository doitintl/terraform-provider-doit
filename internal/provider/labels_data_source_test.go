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

// TestAccLabelsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccLabelsDataSource_MaxResultsOnly(t *testing.T) {
	labelCount := getLabelCount(t)
	if labelCount < 3 {
		t.Skipf("Need at least 3 labels to test pagination, got %d", labelCount)
	}

	resource.ParallelTest(t, resource.TestCase{
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
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccLabelsDataSourceMaxResultsConfig("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
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

// TestAccLabelsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
// Uses three chained data sources in one apply to avoid race conditions with parallel tests:
//   - "all": fetches all labels to get total count
//   - "first_page": fetches 1 label + page_token
//   - "from_token": auto-paginates from the token (should return fewer than all)
func TestAccLabelsDataSource_PageTokenOnly(t *testing.T) {
	totalLabels := getLabelCount(t)
	if totalLabels < 2 {
		t.Skipf("Need at least 2 labels to test page_token-only, got %d", totalLabels)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_labels" "all" {}
data "doit_labels" "first_page" {
  max_results = "1"
}
data "doit_labels" "from_token" {
  page_token = data.doit_labels.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_labels.first_page", "labels.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_labels.first_page", "page_token"),
					testCheckResourceAttrLessThanAttr(
						"data.doit_labels.from_token", "row_count",
						"data.doit_labels.all", "row_count",
					),
					resource.TestCheckNoResourceAttr("data.doit_labels.from_token", "page_token"),
				),
			},
		},
	})
}

// TestAccLabelsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccLabelsDataSource_MaxResultsAndPageToken(t *testing.T) {
	labelCount := getLabelCount(t)
	if labelCount < 3 {
		t.Skipf("Need at least 3 labels to test pagination, got %d", labelCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_labels" "first_page" {
  max_results = "1"
}
data "doit_labels" "paginated" {
  max_results = "1"
  page_token  = data.doit_labels.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_labels.first_page", "labels.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_labels.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_labels.paginated", "labels.#", "1"),
				),
			},
		},
	})
}

// TestAccLabelsDataSource_AutoPagination tests that without max_results, all labels are fetched.
func TestAccLabelsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_labels.test", "row_count"),
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

var (
	labelCount     int
	labelCountOnce sync.Once
)

func getLabelCount(t *testing.T) int {
	t.Helper()
	labelCountOnce.Do(func() {
		labelCount = computeLabelCount(t)
	})
	return labelCount
}

func computeLabelCount(t *testing.T) int {
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
