package provider_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLabel(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-label")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Label Create
			{
				Config: testAccLabel(rName),
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
						knownvalue.StringExact(rName)),
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
				Config: testAccLabelUpdate(rName),
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
						knownvalue.StringExact(rName+"-updated")),
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
	rName := acctest.RandomWithPrefix("tf-acc-label")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabel(rName),
			},
			{
				ResourceName:      "doit_label.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLabel(name string) string {
	return fmt.Sprintf(`
resource "doit_label" "this" {
  name  = %q
  color = "blue"
}
`, name)
}

func testAccLabelUpdate(name string) string {
	return fmt.Sprintf(`
resource "doit_label" "this" {
  name  = "%s-updated"
  color = "mint"
}
`, name)
}

// TestAccLabel_Lavender tests that the API accepts the "lavender" color.
func TestAccLabel_Lavender(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-lavender")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_label" "lavender_test" {
  name  = %q
  color = "lavender"
}
`, rName),
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
	rName := acctest.RandomWithPrefix("tf-acc-invalid")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "doit_label" "invalid_test" {
  name  = %q
  color = "invalid_color"
}
`, rName),
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

	rName := acctest.RandomWithPrefix("tf-acc-label")
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccLabel(rName),
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
				Config:             testAccLabel(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}
