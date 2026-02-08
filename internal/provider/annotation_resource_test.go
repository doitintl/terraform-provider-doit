package provider_test

import (
	"context"
	"fmt"
	"regexp"
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

	resource.ParallelTest(t, resource.TestCase{
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

	resource.ParallelTest(t, resource.TestCase{
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

	resource.ParallelTest(t, resource.TestCase{
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

	resource.ParallelTest(t, resource.TestCase{
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

	resource.ParallelTest(t, resource.TestCase{
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

// TestAccAnnotation_WithEmptyLists tests that explicit empty lists [] are handled correctly
// and don't cause "inconsistent result" errors.
func TestAccAnnotation_WithEmptyLists(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationWithEmptyLists(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.empty_lists",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Annotation with empty lists %d", n))),
					statecheck.ExpectKnownValue(
						"doit_annotation.empty_lists",
						tfjsonpath.New("labels"),
						knownvalue.ListSizeExact(0)),
					statecheck.ExpectKnownValue(
						"doit_annotation.empty_lists",
						tfjsonpath.New("reports"),
						knownvalue.ListSizeExact(0)),
				},
			},
		},
	})
}

func testAccAnnotationWithEmptyLists(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "empty_lists" {
  content   = "Annotation with empty lists %d"
  timestamp = "%s"
  labels    = []
  reports   = []
}
`, i, timestamp)
}

// TestAccAnnotation_WithOmittedLists tests that omitted lists are handled correctly.
// Note: The API returns empty arrays even when labels/reports are not sent, so the
// state will contain empty lists rather than null.
func TestAccAnnotation_WithOmittedLists(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationWithOmittedLists(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.omitted_lists",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Annotation with omitted lists %d", n))),
					// API returns empty arrays even when lists are not sent
					statecheck.ExpectKnownValue(
						"doit_annotation.omitted_lists",
						tfjsonpath.New("labels"),
						knownvalue.ListSizeExact(0)),
					statecheck.ExpectKnownValue(
						"doit_annotation.omitted_lists",
						tfjsonpath.New("reports"),
						knownvalue.ListSizeExact(0)),
				},
			},
		},
	})
}

func testAccAnnotationWithOmittedLists(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "omitted_lists" {
  content   = "Annotation with omitted lists %d"
  timestamp = "%s"
}
`, i, timestamp)
}

// TestAccAnnotation_TimezonePreservation tests that timestamps with non-UTC timezone
// offsets are preserved correctly, avoiding "Provider produced inconsistent result" errors.
func TestAccAnnotation_TimezonePreservation(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	// Use EST timezone offset (-05:00) instead of UTC
	timestamp := "2024-06-15T12:00:00-05:00"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationWithTimezone(n, timestamp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_annotation.timezone_test",
						tfjsonpath.New("content"),
						knownvalue.StringExact(fmt.Sprintf("Annotation with timezone %d", n))),
					// Verify the timestamp is preserved with the original timezone offset
					statecheck.ExpectKnownValue(
						"doit_annotation.timezone_test",
						tfjsonpath.New("timestamp"),
						knownvalue.StringExact(timestamp)),
				},
			},
		},
	})
}

func testAccAnnotationWithTimezone(i int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "timezone_test" {
  content   = "Annotation with timezone %d"
  timestamp = "%s"
}
`, i, timestamp)
}

// TestAccAnnotation_InvalidTimestamp tests that invalid timestamp formats are rejected
// at plan time by the RFC3339 validator.
func TestAccAnnotation_InvalidTimestamp(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAnnotationInvalidTimestamp(),
				ExpectError: regexp.MustCompile(`Invalid RFC3339 Timestamp`),
			},
		},
	})
}

func testAccAnnotationInvalidTimestamp() string {
	return `
resource "doit_annotation" "invalid" {
  content   = "Test annotation with invalid timestamp"
  timestamp = "not-a-valid-timestamp"
}
`
}

// TestAccAnnotation_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccAnnotation_Disappears(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccAnnotation(n, timestamp),
				Check: resource.ComposeTestCheckFunc(
					// Capture the resource ID for later deletion
					resource.TestCheckResourceAttrWith("doit_annotation.this", "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("resource ID is empty")
						}
						resourceId = value
						return nil
					}),
				),
			},
			// Step 2: Delete the resource via API, then verify Terraform detects the drift
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteAnnotationWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete annotation via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204, or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccAnnotation(n, timestamp),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}
