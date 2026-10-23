package provider_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
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
//
// It creates its own annotations so the result does not depend on the tenant's existing data.
// The fixtures are built so that each field yields a different order, which means an API that
// sorted by the wrong field (or ignored sort_by) would fail at least one check:
//
//	created (chained):  A, B, C
//	timestamp:          B < C < A
//	modified (step 2 updates B): A < C < B
//
// Other tests running in parallel may add annotations to the tenant, so the checks only look at
// the relative order of the fixtures, picked out by a per-test content prefix.
func TestAccAnnotationsDataSource_SortByOrdering(t *testing.T) {
	n := acctest.RandInt()
	prefix := fmt.Sprintf("sort-by-ordering-%d", n)
	now := time.Now().UTC()
	timestamps := map[string]string{
		"A": now.AddDate(0, 0, -2).Format(time.RFC3339),
		"B": now.AddDate(0, 0, -4).Format(time.RFC3339),
		"C": now.AddDate(0, 0, -3).Format(time.RFC3339),
	}

	const ds = "data.doit_annotations.%s_%s"
	checks := func(created, timestamp, modified []string) resource.TestCheckFunc {
		var fns []resource.TestCheckFunc
		for _, tc := range []struct {
			sortBy string
			want   []string
		}{
			{"timestamp", timestamp},
			{"timeCreated", created},
			{"timeModified", modified},
		} {
			fns = append(fns,
				testAccCheckAnnotationFixtureOrder(fmt.Sprintf(ds, tc.sortBy, "asc"), prefix, tc.want),
				testAccCheckAnnotationFixtureOrder(fmt.Sprintf(ds, tc.sortBy, "desc"), prefix, reversed(tc.want)),
			)
		}
		return resource.ComposeAggregateTestCheckFunc(fns...)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Never modified, so update_time is not asserted on in this step.
				Config: testAccAnnotationsSortOrderingConfig(prefix, timestamps, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAnnotationFixtureOrder("data.doit_annotations.timeCreated_asc", prefix, []string{"A", "B", "C"}),
					testAccCheckAnnotationFixtureOrder("data.doit_annotations.timeCreated_desc", prefix, []string{"C", "B", "A"}),
					testAccCheckAnnotationFixtureOrder("data.doit_annotations.timestamp_asc", prefix, []string{"B", "C", "A"}),
					testAccCheckAnnotationFixtureOrder("data.doit_annotations.timestamp_desc", prefix, []string{"A", "C", "B"}),
				),
			},
			{
				Config: testAccAnnotationsSortOrderingConfig(prefix, timestamps, true),
				Check:  checks([]string{"A", "B", "C"}, []string{"B", "C", "A"}, []string{"A", "C", "B"}),
			},
		},
	})
}

func reversed(keys []string) []string {
	r := slices.Clone(keys)
	slices.Reverse(r)
	return r
}

// testAccCheckAnnotationFixtureOrder checks that the annotations whose content starts with prefix
// appear in the data source in the order given by keys. The key of an annotation is the first
// token after "<prefix>-" in its content.
func testAccCheckAnnotationFixtureOrder(name, prefix string, keys []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		attrs := rs.Primary.Attributes

		count, err := strconv.Atoi(attrs["annotations.#"])
		if err != nil {
			return fmt.Errorf("parsing annotations.#: %w", err)
		}

		var got []string
		for i := range count {
			content := attrs[fmt.Sprintf("annotations.%d.content", i)]
			if rest, found := strings.CutPrefix(content, prefix+"-"); found {
				got = append(got, strings.Fields(rest)[0])
			}
		}
		if !slices.Equal(got, keys) {
			return fmt.Errorf("%s: fixtures in order %v, want %v", name, got, keys)
		}
		return nil
	}
}

// testAccAnnotationsSortOrderingConfig creates three annotations in sequence (A, B, C) and reads
// them back with every sort_by/sort_order combination. When modifyB is set, B's content is
// changed, which bumps its update_time past C's.
func testAccAnnotationsSortOrderingConfig(prefix string, timestamps map[string]string, modifyB bool) string {
	suffix := ""
	if modifyB {
		suffix = " (modified)"
	}

	var b strings.Builder
	prev := ""
	for _, key := range []string{"A", "B", "C"} {
		dependsOn := ""
		if prev != "" {
			dependsOn = fmt.Sprintf("\n  depends_on = [doit_annotation.%s]", prev)
		}
		content := fmt.Sprintf("%s-%s", prefix, key)
		if key == "B" {
			content += suffix
		}
		fmt.Fprintf(&b, `
resource "doit_annotation" "%s" {
  content   = "%s"
  timestamp = "%s"%s
}
`, strings.ToLower(key), content, timestamps[key], dependsOn)
		prev = strings.ToLower(key)
	}

	for _, sortBy := range []string{"timestamp", "timeCreated", "timeModified"} {
		for _, order := range []string{"asc", "desc"} {
			fmt.Fprintf(&b, `
data "doit_annotations" "%s_%s" {
  sort_by    = "%s"
  sort_order = "%s"
  depends_on = [doit_annotation.a, doit_annotation.b, doit_annotation.c]
}
`, sortBy, order, sortBy, order)
		}
	}
	return b.String()
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
