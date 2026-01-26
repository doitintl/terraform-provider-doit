package provider_test

import (
	"fmt"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAnnotation(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Annotation Create
			{
				Config: testAccAnnotation(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_annotation.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.this",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Test annotation content %d", n))),
					statecheck.ExpectKnownValue(
						"doit_annotation.this",
						tfjsonpath.New("timestamp"),
						knownvalue.StringExact(timestamp)),
				},
			},
			// Test Annotation Update (change content)
			{
				Config: testAccAnnotationUpdate(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_annotation.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.this",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Updated annotation content %d", n))),
				},
			},
		},
	})
}

func TestAccAnnotation_Import(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotation(n, timestamp),
			},
			{
				ResourceName:      "doit_annotation.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAnnotation(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "this" {
  content   = "Test annotation content %d"
  timestamp = "%s"
}
`, i, timestamp)
}

func testAccAnnotationUpdate(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "this" {
  content   = "Updated annotation content %d"
  timestamp = "%s"
}
`, i, timestamp)
}

func TestAccAnnotation_WithReport(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Create annotation with associated report
			{
				Config: testAccAnnotationWithReport(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.with_report",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Annotation with report %d", n))),
					statecheck.ExpectKnownValue(
						"doit_annotation.with_report",
						tfjsonpath.New("reports"),
						knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

func testAccAnnotationWithReport(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
  name = "test-report-for-annotation-%d"
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

resource "doit_annotation" "with_report" {
  content   = "Annotation with report %d"
  timestamp = "%s"
  reports   = [doit_report.test.id]
}
`, i, i, timestamp)
}

// TestAccAnnotation_WithLabelsAndReports tests setting both labels and reports simultaneously.
func TestAccAnnotation_WithLabelsAndReports(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationWithLabelsAndReports(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.with_both",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Annotation with labels and reports %d", n))),
					statecheck.ExpectKnownValue(
						"doit_annotation.with_both",
						tfjsonpath.New("labels"),
						knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(
						"doit_annotation.with_both",
						tfjsonpath.New("reports"),
						knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

func testAccAnnotationWithLabelsAndReports(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "test-label-for-annotation-%d"
  color = "blue"
}

resource "doit_report" "test" {
  name = "test-report-for-annotation-both-%d"
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

resource "doit_annotation" "with_both" {
  content   = "Annotation with labels and reports %d"
  timestamp = "%s"
  labels    = [doit_label.test.id]
  reports   = [doit_report.test.id]
}
`, i, i, i, timestamp)
}

// TestAccAnnotation_WithMultipleLabelsAndReports tests setting multiple labels and reports.
func TestAccAnnotation_WithMultipleLabelsAndReports(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationWithMultipleLabelsAndReports(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.with_multiple",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Annotation with multiple labels and reports %d", n))),
					statecheck.ExpectKnownValue(
						"doit_annotation.with_multiple",
						tfjsonpath.New("labels"),
						knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue(
						"doit_annotation.with_multiple",
						tfjsonpath.New("reports"),
						knownvalue.ListSizeExact(2)),
				},
			},
		},
	})
}

func testAccAnnotationWithMultipleLabelsAndReports(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_label" "first" {
  name  = "test-label-first-%d"
  color = "blue"
}

resource "doit_label" "second" {
  name  = "test-label-second-%d"
  color = "mint"
}

resource "doit_report" "first" {
  name = "test-report-first-%d"
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

resource "doit_report" "second" {
  name = "test-report-second-%d"
  config = {
    metric = {
      type  = "basic"
      value = "usage"
    }
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}

resource "doit_annotation" "with_multiple" {
  content   = "Annotation with multiple labels and reports %d"
  timestamp = "%s"
  labels    = [doit_label.first.id, doit_label.second.id]
  reports   = [doit_report.first.id, doit_report.second.id]
}
`, i, i, i, i, i, timestamp)
}
