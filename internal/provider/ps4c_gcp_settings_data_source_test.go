package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpSettingsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpSettingsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_settings.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_settings.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_settings.test", "items.0.services.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpSettingsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccPs4cGcpSettingsDataSourceConfig() string {
	return `
data "doit_ps4c_gcp_settings" "test" {
  lifecycle {
    postcondition {
      condition = alltrue(flatten([
        for account in self.items : [
          for item in account.services :
          item.region != "" && (
            item.service == "compute" ? item.region == "global" :
            item.service == "cloud_sql" ? item.region != "global" : false
          )
        ]
      ]))
      error_message = "PS4C GCP settings must expose global compute or regional cloud_sql scopes."
    }
  }
}
`
}
