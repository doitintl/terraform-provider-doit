package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccActiveTheme exercises the full lifecycle: create → update → drift check.
func TestAccActiveTheme(t *testing.T) {
	rName1 := acctest.RandomWithPrefix("tf-acc-active-theme-a")
	rName2 := acctest.RandomWithPrefix("tf-acc-active-theme-b")

	// Singleton resource — cannot run in parallel with other active theme tests.
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton resource: parallel tests interfere via shared active theme
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a custom theme and set it as active
			{
				Config: testAccActiveThemeConfig(rName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_active_theme.this", "id", "active-theme"),
					resource.TestCheckResourceAttrPair(
						"doit_active_theme.this", "theme_id",
						"doit_custom_theme.a", "id"),
				),
			},
			// Step 2: Update to a different theme
			{
				Config: testAccActiveThemeUpdatedConfig(rName1, rName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"doit_active_theme.this", "theme_id",
						"doit_custom_theme.b", "id"),
				),
			},
			// Step 3: Drift check
			{
				Config: testAccActiveThemeUpdatedConfig(rName1, rName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccActiveTheme_Import tests import + drift check.
func TestAccActiveTheme_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-active-theme-imp")

	// Singleton resource — cannot run in parallel with other active theme tests.
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton resource: parallel tests interfere via shared active theme
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccActiveThemeConfig(rName),
			},
			// Step 2: Import
			{
				ResourceName:      "doit_active_theme.this",
				ImportState:       true,
				ImportStateId:     "active-theme",
				ImportStateVerify: true,
			},
			// Step 3: Drift check
			{
				Config: testAccActiveThemeConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// --- Test config helpers ---

func testAccActiveThemeConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "a" {
  name          = %q
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}

resource "doit_active_theme" "this" {
  theme_id = doit_custom_theme.a.id
}
`, name)
}

func testAccActiveThemeUpdatedConfig(name1, name2 string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "a" {
  name          = %q
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}

resource "doit_custom_theme" "b" {
  name          = %q
  primary_color = "#34A853"

  colors = {
    light = ["#34A853", "#FBBC04", "#EA4335"]
    dark  = ["#81C995", "#FDD663", "#F28B82"]
  }
}

resource "doit_active_theme" "this" {
  theme_id = doit_custom_theme.b.id
}
`, name1, name2)
}
