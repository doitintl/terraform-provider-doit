package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccRolesDataSource_Basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-roles-ds")
	config := testAccRolesDataSourceConfig(name)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "roles.#"),
					resource.TestCheckResourceAttrSet("data.doit_roles.test", "row_count"),
					resource.TestCheckOutput("list_contains_role", "true"),
					resource.TestCheckOutput("list_matches_role", "true"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccRolesDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_role" "test" {
  name        = %q
  description = "Roles data source acceptance test"
  permissions = []
}

data "doit_roles" "test" {
  depends_on = [doit_role.test]
}

locals {
  matching_roles = [for r in data.doit_roles.test.roles : r if r.id == doit_role.test.id]
}

output "list_contains_role" {
  value = length(local.matching_roles) == 1
}

output "list_matches_role" {
  value = alltrue([for r in local.matching_roles :
    r.name == doit_role.test.name &&
    r.description == doit_role.test.description &&
    r.permissions == doit_role.test.permissions &&
    r.type == "custom" &&
    r.customer != null &&
    r.child_tenant_eligible != null
  ])
}
`, name)
}
