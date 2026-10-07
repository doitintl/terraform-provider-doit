package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccWidgetsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccWidgetsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_widgets.all", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_widgets.all", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_widgets.all", "items.0.alias"),
					resource.TestCheckResourceAttrSet("data.doit_widgets.all", "items.0.name"),
					resource.TestCheckResourceAttrSet("data.doit_widgets.all", "items.0.description"),
					resource.TestCheckResourceAttr("data.doit_widgets.all", "items.0.type", "preset"),
					resource.TestCheckResourceAttr("data.doit_widgets.all", "items.0.kind", "monetary_metric"),
					resource.TestCheckResourceAttrSet("data.doit_widgets.all", "row_count"),
				),
			},
			{
				Config: testAccWidgetsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccWidgetsDataSourceConfig() string {
	return `
data "doit_widgets" "all" {}
`
}
