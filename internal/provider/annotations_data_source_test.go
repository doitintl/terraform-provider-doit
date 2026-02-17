package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
func TestAccAnnotationsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): The annotations API ignores pageToken when maxResults is not set, returning all results.
	// Remove this skip once the API supports page_token-only pagination.
	t.Skip("Skipped: annotations API ignores pageToken without maxResults (CMP-38591)")

	totalAnnotations := getAnnotationCount(t)
	if totalAnnotations < 2 {
		t.Skipf("Need at least 2 annotations to test page_token-only, got %d", totalAnnotations)
	}

	pageToken := getAnnotationFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 annotation)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_annotations.from_token", "annotations.#"),
					testCheckResourceAttrLessThan("data.doit_annotations.from_token", "row_count", totalAnnotations),
				),
			},
		},
	})
}

func testAccAnnotationsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_annotations" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccAnnotationsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccAnnotationsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getAnnotationFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 annotation)")
	}

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
				Config: testAccAnnotationsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_annotations.paginated", "annotations.#", "1"),
				),
			},
		},
	})
}

func testAccAnnotationsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_annotations" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
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

func getAnnotationFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListAnnotationsWithResponse(ctx, &models.ListAnnotationsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list annotations: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
