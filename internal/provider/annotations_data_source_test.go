package provider_test

import (
	"context"
	"fmt"
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

	resource.Test(t, resource.TestCase{
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

// TestAccAnnotationsDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccAnnotationsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getAnnotationFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 annotation)")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_annotations.from_token", "annotations.#"),
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

	resource.Test(t, resource.TestCase{
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
	expectedCount := getAnnotationCount(t)
	if expectedCount == 0 {
		t.Skip("No annotations available to test auto-pagination")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_annotations.test", "row_count", fmt.Sprintf("%d", expectedCount)),
					resource.TestCheckNoResourceAttr("data.doit_annotations.test", "page_token"),
				),
			},
		},
	})
}

func testAccAnnotationsDataSourceConfig() string {
	return `
data "doit_annotations" "test" {}
`
}

// Helper functions

func getAnnotationCount(t *testing.T) int {
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
