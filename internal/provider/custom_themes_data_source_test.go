package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCustomThemesDataSource_Basic(t *testing.T) {
	// Create a theme so the list is guaranteed non-empty, then list all.
	rName := acctest.RandomWithPrefix("tf-acc-themes-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomThemesDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_custom_themes.all", "row_count"),
				),
			},
			// Drift verification
			{
				Config: testAccCustomThemesDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCustomThemesDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "seed" {
  name          = %q
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}

data "doit_custom_themes" "all" {
  depends_on = [doit_custom_theme.seed]
}
`, name)
}
