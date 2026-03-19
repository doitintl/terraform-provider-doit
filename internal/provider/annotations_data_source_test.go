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
// Uses chained data sources to avoid page token expiry.
func TestAccAnnotationsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): API ignores pageToken when maxResults is not set.
	t.Skip("Skipping: API ignores pageToken without maxResults (CMP-38591)")
	totalAnnotations := getAnnotationCount(t)
	if totalAnnotations < 2 {
		t.Skipf("Need at least 2 annotations to test page_token-only, got %d", totalAnnotations)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsDataSourceChainedPageTokenOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_annotations.from_token", "annotations.#"),
					testCheckResourceAttrLessThan("data.doit_annotations.from_token", "row_count", totalAnnotations),
				),
			},
		},
	})
}

func testAccAnnotationsDataSourceChainedPageTokenOnly() string {
	return `
data "doit_annotations" "first_page" {
  max_results = "1"
}
data "doit_annotations" "from_token" {
  page_token = data.doit_annotations.first_page.page_token
}
`
}

// TestAccAnnotationsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses chained data sources to avoid page token expiry.
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
				Config: testAccAnnotationsDataSourceChainedMaxResultsAndPageToken(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_annotations.second_page", "annotations.#", "1"),
				),
			},
		},
	})
}

func testAccAnnotationsDataSourceChainedMaxResultsAndPageToken() string {
	return `
data "doit_annotations" "first_page" {
  max_results = "1"
}
data "doit_annotations" "second_page" {
  max_results = "1"
  page_token  = data.doit_annotations.first_page.page_token
}
`
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
