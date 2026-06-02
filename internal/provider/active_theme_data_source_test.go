package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccActiveThemeDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-active-theme-ds")

	// Singleton resource — cannot run in parallel with other active theme tests.
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton: parallel tests interfere via shared active theme
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccActiveThemeDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_active_theme.test", "theme_id",
						"doit_custom_theme.test", "id"),
				),
			},
			// Drift verification
			{
				Config: testAccActiveThemeDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccActiveThemeDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "test" {
  name          = %q
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}

resource "doit_active_theme" "test" {
  theme_id = doit_custom_theme.test.id
}

data "doit_active_theme" "test" {
  depends_on = [doit_active_theme.test]
}
`, name)
}

func TestAccActiveThemeDataSource_Default(t *testing.T) {
	// Explicitly reset to built-in default, then verify the data source reads "default".
	// Singleton resource — cannot run in parallel with other active theme tests.
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton: parallel tests interfere via shared active theme
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccActiveThemeDataSourceDefaultConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_active_theme.current", "theme_id", "default"),
				),
			},
		},
	})
}

func testAccActiveThemeDataSourceDefaultConfig() string {
	return `
resource "doit_active_theme" "reset" {
  theme_id = "default"
}

data "doit_active_theme" "current" {
  depends_on = [doit_active_theme.reset]
}
`
}
