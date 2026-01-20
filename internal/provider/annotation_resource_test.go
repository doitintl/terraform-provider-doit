package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
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

	// Fetch an existing label ID from the API
	labelID := getValidLabelID(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationWithLabelsAndReports(n, timestamp, labelID),
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

func getValidLabelID(t *testing.T) string {
	token := os.Getenv("DOIT_API_TOKEN")
	host := os.Getenv("DOIT_HOST")
	customerContext := os.Getenv("DOIT_CUSTOMER_CONTEXT")

	if token == "" || host == "" {
		t.Skip("DOIT_API_TOKEN and DOIT_HOST must be set for dynamic ID fetching")
	}

	ctx := context.Background()

	client, err := provider.NewClient(ctx, host, token, customerContext)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// List Labels
	labelResp, err := client.ListLabels(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list labels: %v", err)
	}
	defer func() {
		if closeErr := labelResp.Body.Close(); closeErr != nil {
			t.Logf("[WARN] Error closing response body: %v", closeErr)
		}
	}()

	var labelResult map[string]interface{}
	if decodeErr := json.NewDecoder(labelResp.Body).Decode(&labelResult); decodeErr != nil {
		t.Fatalf("Failed to decode labels response: %v", decodeErr)
	}

	var labelID string
	if labels, ok := labelResult["labels"].([]interface{}); ok && len(labels) > 0 {
		if lMap, ok := labels[0].(map[string]interface{}); ok {
			if id, ok := lMap["id"].(string); ok {
				labelID = id
			}
		}
	}

	if labelID == "" {
		t.Skip("No valid labels found to test with - create a label first")
	}

	return labelID
}

func testAccAnnotationWithLabelsAndReports(i int, timestamp, labelID string) string {
	return fmt.Sprintf(`
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
  labels    = ["%s"]
  reports   = [doit_report.test.id]
}
`, i, i, timestamp, labelID)
}
