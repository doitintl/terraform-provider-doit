package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCustomThemeDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-theme-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomThemeDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_custom_theme.test", "id",
						"doit_custom_theme.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_custom_theme.test", "name",
						"doit_custom_theme.test", "name"),
					resource.TestCheckResourceAttrPair(
						"data.doit_custom_theme.test", "primary_color",
						"doit_custom_theme.test", "primary_color"),
					resource.TestCheckResourceAttrSet(
						"data.doit_custom_theme.test", "create_time"),
					resource.TestCheckResourceAttr(
						"data.doit_custom_theme.test", "colors.light.#", "3"),
					resource.TestCheckResourceAttr(
						"data.doit_custom_theme.test", "colors.dark.#", "3"),
				),
			},
			// Drift verification
			{
				Config: testAccCustomThemeDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCustomThemeDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "test" {
  name          = %q
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}

data "doit_custom_theme" "test" {
  id = doit_custom_theme.test.id
}
`, name)
}

func TestAccCustomThemeDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCustomThemeDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading custom theme|not found|404`),
			},
		},
	})
}

func testAccCustomThemeDataSourceNotFoundConfig() string {
	return `
data "doit_custom_theme" "notfound" {
  id = "nonexistent-theme-id"
}
`
}
