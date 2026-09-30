package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRolesDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccRolesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "roles.#"),
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "roles.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "roles.0.name"),
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "roles.0.type"),
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "roles.0.permissions.#"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_roles.test",
						tfjsonpath.New("roles").AtSliceIndex(0).AtMapKey("customer"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.doit_roles.test",
						tfjsonpath.New("roles").AtSliceIndex(0).AtMapKey("description"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.doit_roles.test",
						tfjsonpath.New("roles").AtSliceIndex(0).AtMapKey("child_tenant_eligible"),
						knownvalue.NotNull(),
					),
				},
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccRolesDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccRolesDataSourceConfig() string {
	return `
data "doit_roles" "test" {}
`
}
