package provider_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAnnotationDataSource_Basic(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationDataSourceConfig(n, timestamp),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "id",
						"doit_annotation.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "content",
						"doit_annotation.test", "content"),
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "timestamp",
						"doit_annotation.test", "timestamp"),
				),
			},
		},
	})
}

func testAccAnnotationDataSourceConfig(n int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "test" {
  content   = "Test annotation content %d"
  timestamp = "%s"
}

data "doit_annotation" "test" {
  id = doit_annotation.test.id
}
`, n, timestamp)
}

func TestAccAnnotationDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAnnotationDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading annotation|not found|404`),
			},
		},
	})
}

func testAccAnnotationDataSourceNotFoundConfig() string {
	return `
data "doit_annotation" "notfound" {
  id = "nonexistent-annotation-id"
}
`
}

// TestAccAnnotationDataSource_WithLabelsAndReports tests reading an annotation
// with labels and reports to cover the optional field mapping paths.
func TestAccAnnotationDataSource_WithLabelsAndReports(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationDataSourceWithLabelsAndReports(n, timestamp),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "id",
						"doit_annotation.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "content",
						"doit_annotation.test", "content"),
					// Verify labels come through correctly
					resource.TestCheckResourceAttr(
						"data.doit_annotation.test", "labels.#", "1"),
					resource.TestCheckResourceAttrSet(
						"data.doit_annotation.test", "labels.0.id"),
					resource.TestCheckResourceAttrSet(
						"data.doit_annotation.test", "labels.0.name"),
					// Verify reports come through correctly
					resource.TestCheckResourceAttr(
						"data.doit_annotation.test", "reports.#", "1"),
					// Verify computed timestamps are present
					resource.TestCheckResourceAttrSet(
						"data.doit_annotation.test", "create_time"),
				),
			},
		},
	})
}

func testAccAnnotationDataSourceWithLabelsAndReports(n int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "test-label-for-ds-%d"
  color = "blue"
}

resource "doit_report" "test" {
  name = "test-report-for-ds-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}

resource "doit_annotation" "test" {
  content   = "Annotation with labels and reports for DS test %d"
  timestamp = "%s"
  labels    = [doit_label.test.id]
  reports   = [doit_report.test.id]
}

data "doit_annotation" "test" {
  id = doit_annotation.test.id
}
`, n, n, n, timestamp)
}
