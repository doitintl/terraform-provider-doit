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

// TestAccCustomTheme exercises the full lifecycle: create → update → drift check.
func TestAccCustomTheme(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-theme")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccCustomTheme(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_custom_theme.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("primary_color"),
						knownvalue.StringExact("#1A73E8")),
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("colors").AtMapKey("light"),
						knownvalue.ListSizeExact(3)),
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("colors").AtMapKey("dark"),
						knownvalue.ListSizeExact(3)),
				},
			},
			// Step 2: Update (change name, primary_color, and colors)
			{
				Config: testAccCustomThemeUpdated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_custom_theme.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName+"-updated")),
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("primary_color"),
						knownvalue.StringExact("#34A853")),
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("colors").AtMapKey("light"),
						knownvalue.ListSizeExact(4)),
					statecheck.ExpectKnownValue(
						"doit_custom_theme.this",
						tfjsonpath.New("colors").AtMapKey("dark"),
						knownvalue.ListSizeExact(4)),
				},
			},
			// Step 3: Drift check — re-apply same updated config, expect no changes.
			{
				Config: testAccCustomThemeUpdated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccCustomTheme_Import tests import + drift check.
func TestAccCustomTheme_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-theme")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccCustomTheme(rName),
			},
			// Step 2: Import
			{
				ResourceName:      "doit_custom_theme.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Drift check — re-apply config after import, expect no changes.
			{
				Config: testAccCustomTheme(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccCustomTheme_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
func TestAccCustomTheme_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-theme")
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccCustomTheme(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("doit_custom_theme.this", "id", func(value string) error {
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
					resp, err := client.DeleteCustomThemeWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete custom theme via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccCustomTheme(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}

// TestAccCustomTheme_InvalidColor tests that invalid hex colors are rejected at the schema level.
func TestAccCustomTheme_InvalidColor(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-theme")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Invalid primary_color
			{
				Config: fmt.Sprintf(`
resource "doit_custom_theme" "invalid" {
  name          = %q
  primary_color = "not-a-hex"

  colors = {
    light = ["#FF0000"]
    dark  = ["#00FF00"]
  }
}
`, rName),
				ExpectError: regexp.MustCompile(`value must match regular expression`),
			},
			// Valid primary_color but invalid hex in colors.dark
			{
				Config: fmt.Sprintf(`
resource "doit_custom_theme" "invalid" {
  name          = %q
  primary_color = "#FF0000"

  colors = {
    light = ["#FF0000"]
    dark  = ["not-a-hex"]
  }
}
`, rName),
				ExpectError: regexp.MustCompile(`must be a hex color`),
			},
		},
	})
}

// --- Test config helpers ---

func testAccCustomTheme(name string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "this" {
  name          = %q
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}
`, name)
}

func testAccCustomThemeUpdated(name string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "this" {
  name          = "%s-updated"
  primary_color = "#34A853"

  colors = {
    light = ["#34A853", "#FBBC04", "#EA4335", "#A142F4"]
    dark  = ["#81C995", "#FDD663", "#F28B82", "#C58AF9"]
  }
}
`, name)
}
