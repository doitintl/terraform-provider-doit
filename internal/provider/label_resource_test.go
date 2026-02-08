package provider_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLabel(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Label Create
			{
				Config: testAccLabel(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_label.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-label-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("color"),
						knownvalue.StringExact("blue")),
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("custom")),
				},
			},
			// Test Label Update (change name and color)
			{
				Config: testAccLabelUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_label.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-label-updated-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_label.this",
						tfjsonpath.New("color"),
						knownvalue.StringExact("mint")),
				},
			},
		},
	})
}

func TestAccLabel_Import(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabel(n),
			},
			{
				ResourceName:      "doit_label.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "this" {
  name  = "test-label-%d"
  color = "blue"
}
`, i)
}

func testAccLabelUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "this" {
  name  = "test-label-updated-%d"
  color = "mint"
}
`, i)
}

// TestAccLabel_Lavender tests that the API accepts the "lavender" color.
func TestAccLabel_Lavender(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_label" "lavender_test" {
  name  = "test-lavender-%d"
  color = "lavender"
}
`, n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_label.lavender_test",
						tfjsonpath.New("color"),
						knownvalue.StringExact("lavender")),
				},
			},
		},
	})
}

// TestAccLabel_InvalidColor tests that invalid colors are rejected.
func TestAccLabel_InvalidColor(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_label" "invalid_test" {
  name  = "test-invalid-color-%d"
  color = "invalid_color"
}
`, n),
				ExpectError: regexp.MustCompile(`(?i)value must be one of:`),
			},
		},
	})
}

// TestAccLabel_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccLabel_Disappears(t *testing.T) {
	// Skip until API DELETE returns 404 instead of 500 for non-existent resources
	// See: https://doitintl.atlassian.net/browse/CMP-37040
	t.Skip("Skipping until API DELETE returns 404 instead of 500 (CMP-37040)")

	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccLabel(n),
				Check: resource.ComposeTestCheckFunc(
					// Capture the resource ID for later deletion
					resource.TestCheckResourceAttrWith("doit_label.this", "id", func(value string) error {
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
					resp, err := client.DeleteLabelWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete label via API: %v", err)
					}
					if resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccLabel(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}
