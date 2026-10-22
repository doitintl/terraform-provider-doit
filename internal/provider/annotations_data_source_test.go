package provider_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
// auto-paginates starting from the token, returning a different set of items.
// Uses two chained data sources in one apply:
//   - "first_page": fetches 1 annotation + page_token
//   - "from_token": auto-paginates from the token (should start at a different item)
func TestAccAnnotationsDataSource_PageTokenOnly(t *testing.T) {
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
				Config: `
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
					resource.TestCheckResourceAttrSet("data.doit_annotations.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_annotations.from_token", "page_token"),
					// Verify page_token actually advanced to a different starting point
					testCheckResourceAttrNotEqualAttr(
						"data.doit_annotations.first_page", "annotations.0.id",
						"data.doit_annotations.from_token", "annotations.0.id"),
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
					// Verify page_token actually advanced to a different page
					testCheckResourceAttrNotEqualAttr(
						"data.doit_annotations.first_page", "annotations.0.id",
						"data.doit_annotations.paginated", "annotations.0.id"),
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

// TestAccAnnotationsDataSource_SortByAcceptsAllValues verifies that every documented sort_by value
// is accepted by the API and that sort_order is honored alongside it.
func TestAccAnnotationsDataSource_SortByAcceptsAllValues(t *testing.T) {
	for _, sortBy := range []string{"id", "content", "timestamp", "timeCreated", "timeModified"} {
		t.Run(sortBy, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: testAccAnnotationsDataSourceSortConfig(sortBy, "asc"),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("data.doit_annotations.test", "sort_by", sortBy),
							resource.TestCheckResourceAttr("data.doit_annotations.test", "sort_order", "asc"),
							resource.TestCheckResourceAttrSet("data.doit_annotations.test", "row_count"),
						),
					},
					{
						Config: testAccAnnotationsDataSourceSortConfig(sortBy, "desc"),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("data.doit_annotations.test", "sort_by", sortBy),
							resource.TestCheckResourceAttr("data.doit_annotations.test", "sort_order", "desc"),
							resource.TestCheckResourceAttrSet("data.doit_annotations.test", "row_count"),
						),
					},
					// Drift verification: re-apply the same config should produce an empty plan
					{
						Config: testAccAnnotationsDataSourceSortConfig(sortBy, "desc"),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectEmptyPlan(),
							},
						},
					},
				},
			})
		})
	}
}

// TestAccAnnotationsDataSource_SortByOrdering verifies the documented mapping of the sort_by
// values onto the response fields: timestamp -> timestamp, timeCreated -> create_time and
// timeModified -> update_time, in both directions.
func TestAccAnnotationsDataSource_SortByOrdering(t *testing.T) {
	if getAnnotationCount(t) < 2 {
		t.Skip("Need at least 2 annotations to verify sort ordering")
	}

	for _, tc := range []struct{ sortBy, field string }{
		{"timestamp", "timestamp"},
		{"timeCreated", "create_time"},
		{"timeModified", "update_time"},
	} {
		t.Run(tc.sortBy, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: testAccAnnotationsDataSourceSortConfig(tc.sortBy, "asc"),
						Check:  testAccCheckAnnotationsSortedByTime("data.doit_annotations.test", tc.field, true),
					},
					{
						Config: testAccAnnotationsDataSourceSortConfig(tc.sortBy, "desc"),
						Check:  testAccCheckAnnotationsSortedByTime("data.doit_annotations.test", tc.field, false),
					},
				},
			})
		})
	}
}

// testAccCheckAnnotationsSortedByTime checks that the RFC3339 field of the annotations list is
// monotonic in the requested direction. Ties are allowed.
func testAccCheckAnnotationsSortedByTime(name, field string, ascending bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		attrs := rs.Primary.Attributes

		var count int
		if _, err := fmt.Sscanf(attrs["annotations.#"], "%d", &count); err != nil {
			return fmt.Errorf("parsing annotations.#: %w", err)
		}

		var prev time.Time
		for i := range count {
			key := fmt.Sprintf("annotations.%d.%s", i, field)
			raw, ok := attrs[key]
			if !ok {
				return fmt.Errorf("%s is not set", key)
			}
			cur, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return fmt.Errorf("parsing %s=%q: %w", key, raw, err)
			}
			if i > 0 && ((ascending && cur.Before(prev)) || (!ascending && cur.After(prev))) {
				return fmt.Errorf("annotations not sorted by %s (ascending=%t): index %d is %s, previous is %s",
					field, ascending, i, cur.Format(time.RFC3339), prev.Format(time.RFC3339))
			}
			prev = cur
		}
		return nil
	}
}

func testAccAnnotationsDataSourceSortConfig(sortBy, sortOrder string) string {
	return fmt.Sprintf(`
data "doit_annotations" "test" {
  sort_by    = "%s"
  sort_order = "%s"
}
`, sortBy, sortOrder)
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
	skipIfNoAcc(t)
	annotationCountOnce.Do(func() {
		annotationCount = computeAnnotationCount(t)
	})
	return annotationCount
}

func computeAnnotationCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := t.Context()

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
