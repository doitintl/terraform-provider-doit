package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccRoleDataSource_Basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-role-ds")
	config := testAccRoleDataSourceBasicConfig(name)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.doit_role.test", "id", "doit_role.test", "id"),
					resource.TestCheckResourceAttrPair("data.doit_role.test", "name", "doit_role.test", "name"),
					resource.TestCheckResourceAttrPair("data.doit_role.test", "description", "doit_role.test", "description"),
					resource.TestCheckResourceAttrPair("data.doit_role.test", "type", "doit_role.test", "type"),
					resource.TestCheckResourceAttrPair("data.doit_role.test", "customer", "doit_role.test", "customer"),
					resource.TestCheckResourceAttrPair("data.doit_role.test", "child_tenant_eligible", "doit_role.test", "child_tenant_eligible"),
					resource.TestCheckResourceAttrPair("data.doit_role.test", "permissions.#", "doit_role.test", "permissions.#"),
					resource.TestCheckResourceAttr("data.doit_role.test", "type", "custom"),
					resource.TestCheckResourceAttrSet("data.doit_role.test", "customer"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccRoleDataSourceBasicConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_role" "test" {
  name        = %q
  description = "Role data source acceptance test"
  permissions = []
}

data "doit_role" "test" {
  id = doit_role.test.id
}
`, name)
}

func TestAccRoleDataSource_PresetRole(t *testing.T) {
	config := `
data "doit_roles" "presets" {}

locals {
  preset_role_id = one([
    for r in data.doit_roles.presets.roles : r.id
    if r.type == "preset" && r.name == "View Only"
  ])
}

data "doit_role" "preset" {
  id = local.preset_role_id
}
`
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_role.preset", "type", "preset"),
					resource.TestCheckResourceAttr("data.doit_role.preset", "name", "View Only"),
					resource.TestCheckResourceAttrSet("data.doit_role.preset", "id"),
					resource.TestCheckResourceAttrSet("data.doit_role.preset", "permissions.#"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccRoleDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      `data "doit_role" "missing" { id = "nonexistent-role-id" }`,
			ExpectError: regexp.MustCompile(`(?i)error reading role|404`),
		}},
	})
}
