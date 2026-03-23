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

// TestAccAnnotationsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccAnnotationsDataSource_MaxResultsOnly(t *testing.T) {
	annotationCount := getAnnotationCount(t)
	if annotationCount < 3 {
		t.Skipf("Need at least 3 annotations to test pagination, got %d", annotationCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_annotations.limited", "annotations.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_annotations.limited", "page_token"),
				),
			},
			{
				Config:   testAccAnnotationsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAnnotationsDataSourceMaxResultsConfig("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAnnotationsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_annotations" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccAnnotationsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
// Uses three chained data sources in one apply to avoid race conditions with parallel tests:
//   - "all": fetches all annotations to get total count
//   - "first_page": fetches 1 annotation + page_token
//   - "from_token": auto-paginates from the token (should return fewer than all)
func TestAccAnnotationsDataSource_PageTokenOnly(t *testing.T) {
	totalAnnotations := getAnnotationCount(t)
	if totalAnnotations < 2 {
		t.Skipf("Need at least 2 annotations to test page_token-only, got %d", totalAnnotations)
	}

	// Non-parallel: the "less than" assertion requires a stable resource count
	// during the apply. Parallel tests mutate counts and cause false failures.
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_annotations" "all" {}
data "doit_annotations" "first_page" {
  max_results = "1"
}
data "doit_annotations" "from_token" {
  page_token = data.doit_annotations.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_annotations.first_page", "annotations.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_annotations.first_page", "page_token"),
					testCheckResourceAttrLessThanAttr(
						"data.doit_annotations.from_token", "row_count",
						"data.doit_annotations.all", "row_count",
					),
					resource.TestCheckNoResourceAttr("data.doit_annotations.from_token", "page_token"),
				),
			},
		},
	})
}

// TestAccAnnotationsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccAnnotationsDataSource_MaxResultsAndPageToken(t *testing.T) {
	annotationCount := getAnnotationCount(t)
	if annotationCount < 3 {
		t.Skipf("Need at least 3 annotations to test pagination, got %d", annotationCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_annotations" "first_page" {
  max_results = "1"
}
data "doit_annotations" "paginated" {
  max_results = "1"
  page_token  = data.doit_annotations.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_annotations.first_page", "annotations.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_annotations.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_annotations.paginated", "annotations.#", "1"),
				),
			},
		},
	})
}

// TestAccAnnotationsDataSource_AutoPagination tests that without max_results, all annotations are fetched.
func TestAccAnnotationsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_annotations.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_annotations.test", "page_token"),
				),
			},
		},
	})
}

func testAccAnnotationsDataSourceConfig() string {
	return `
data "doit_annotations" "test" {
}
`
}

// Helper functions

var (
	annotationCount     int
	annotationCountOnce sync.Once
)

func getAnnotationCount(t *testing.T) int {
	t.Helper()
	annotationCountOnce.Do(func() {
		annotationCount = computeAnnotationCount(t)
	})
	return annotationCount
}

func computeAnnotationCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListAnnotationsParams{}
	for {
		resp, err := client.ListAnnotationsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list annotations: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Annotations == nil {
			break
		}
		total += len(*resp.JSON200.Annotations)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
