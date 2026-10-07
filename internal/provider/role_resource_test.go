package provider_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// Permission IDs are opaque and tenant-specific, so the test configs take them
// from the "View Only" preset role. Permission expressions below index into
// local.preset_permissions, e.g. "[local.preset_permissions[0]]".
const (
	rolePermsTwo      = "[local.preset_permissions[0], local.preset_permissions[1]]"
	rolePermsTwoSwap  = "[local.preset_permissions[1], local.preset_permissions[0]]"
	rolePermsThree    = "[local.preset_permissions[0], local.preset_permissions[1], local.preset_permissions[2]]"
	rolePermsEmpty    = "[]"
	roleResourceAddr  = "doit_role.this"
	roleExpectedPerms = "expected_permissions"
)

func TestAccRole_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-role")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccRoleConfig(rName, "Test role", rolePermsTwo),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(roleResourceAddr, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("description"), knownvalue.StringExact("Test role")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("type"), knownvalue.StringExact("custom")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("customer"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("child_tenant_eligible"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("permissions"), knownvalue.ListSizeExact(2)),
				},
				Check: testAccCheckRolePermissionsMatchOutput(roleResourceAddr, roleExpectedPerms),
			},
			// Step 2: Drift check — re-apply same config, expect empty plan
			{
				Config: testAccRoleConfig(rName, "Test role", rolePermsTwo),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Update name, description and permissions
			{
				Config: testAccRoleConfig(rName+"-upd", "Updated description", rolePermsThree),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(roleResourceAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName+"-upd")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("description"), knownvalue.StringExact("Updated description")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("permissions"), knownvalue.ListSizeExact(3)),
				},
				Check: testAccCheckRolePermissionsMatchOutput(roleResourceAddr, roleExpectedPerms),
			},
			// Step 4: Drift check
			{
				Config: testAccRoleConfig(rName+"-upd", "Updated description", rolePermsThree),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRole_ClearableLifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-role-clear")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with populated clearable attributes
			{
				Config: testAccRoleConfig(rName, "Initial description", rolePermsTwo),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("description"), knownvalue.StringExact("Initial description")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("permissions"), knownvalue.ListSizeExact(2)),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccRoleConfig(rName, "Initial description", rolePermsTwo),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear description and permissions by omitting them
			{
				Config: testAccRoleMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleResourceAddr, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("description"), knownvalue.StringExact("")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("permissions"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Step 4: Drift check — cleared values are stable
			{
				Config: testAccRoleMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRole_OmittedOptional(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-role-min")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with only the required name
			{
				Config: testAccRoleMinimalConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("name"), knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("description"), knownvalue.StringExact("")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("permissions"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("type"), knownvalue.StringExact("custom")),
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("child_tenant_eligible"), knownvalue.Bool(false)),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccRoleMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRole_ExplicitEmptyPermissions(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-role-empty")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create explicitly configuring permissions = []
			{
				Config: testAccRoleConfig(rName, "Test empty permissions", rolePermsEmpty),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleResourceAddr, tfjsonpath.New("permissions"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccRoleConfig(rName, "Test empty permissions", rolePermsEmpty),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRole_PermissionsReordering(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-role-reorder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with permissions in order [0, 1]
			{
				Config: testAccRoleConfig(rName, "Test permissions ordering", rolePermsTwo),
				Check:  testAccCheckRolePermissionsMatchOutput(roleResourceAddr, roleExpectedPerms),
			},
			// Step 2: Reorder to [1, 0] — the API preserves input order, so this is an update
			{
				Config: testAccRoleConfig(rName, "Test permissions ordering", rolePermsTwoSwap),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleResourceAddr, plancheck.ResourceActionUpdate),
					},
				},
				Check: testAccCheckRolePermissionsMatchOutput(roleResourceAddr, roleExpectedPerms),
			},
			// Step 3: Drift check — the stored order matches the configured order
			{
				Config: testAccRoleConfig(rName, "Test permissions ordering", rolePermsTwoSwap),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRole_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-role-import")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccRoleConfig(rName, "Import test role", rolePermsTwo),
			},
			// Step 2: Import
			{
				ResourceName:            roleResourceAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			// Step 3: Drift check after import
			{
				Config: testAccRoleConfig(rName, "Import test role", rolePermsTwo),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRole_Validation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccRoleRawConfig(`name = " padded"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must not have leading or trailing whitespace`),
			},
			{
				Config:      testAccRoleRawConfig(`name = "padded "`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must not have leading or trailing whitespace`),
			},
			{
				Config:      testAccRoleRawConfig(`name = ""`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`string length must be at least 1`),
			},
			{
				Config: testAccRoleRawConfig(`
  name        = "tf-acc-role-duplicate-perms"
  permissions = ["perm-a", "perm-a"]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)This attribute contains duplicate values`),
			},
		},
	})
}

func TestAccRole_ImportPresetRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Look up a preset role ID
			{
				Config: testAccRolePresetLookupConfig(),
			},
			// Step 2: Importing it must fail rather than adopt a read-only role
			{
				Config:       testAccRolePresetLookupConfig() + testAccRoleRawConfig(`name = "View Only"`),
				ResourceName: roleResourceAddr,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					out, ok := s.RootModule().Outputs["preset_role_id"]
					if !ok {
						return "", fmt.Errorf("output preset_role_id not found")
					}
					id, ok := out.Value.(string)
					if !ok || id == "" {
						return "", fmt.Errorf("output preset_role_id is not a non-empty string: %#v", out.Value)
					}
					return id, nil
				},
				ExpectError: regexp.MustCompile(`Preset Role Cannot Be Managed`),
			},
		},
	})
}

// testAccCheckRolePermissionsMatchOutput asserts that the resource's
// permissions equal, element by element and in order, the list output that
// the test config computes from the same expression.
func testAccCheckRolePermissionsMatchOutput(resourceName, outputName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		out, ok := s.RootModule().Outputs[outputName]
		if !ok {
			return fmt.Errorf("output %s not found in state", outputName)
		}
		expected, ok := out.Value.([]any)
		if !ok {
			return fmt.Errorf("output %s is not a list: %#v", outputName, out.Value)
		}

		count, err := strconv.Atoi(rs.Primary.Attributes["permissions.#"])
		if err != nil {
			return fmt.Errorf("could not read permissions count: %w", err)
		}
		if count != len(expected) {
			return fmt.Errorf("expected %d permissions, got %d", len(expected), count)
		}
		for i, want := range expected {
			got := rs.Primary.Attributes[fmt.Sprintf("permissions.%d", i)]
			if got != want {
				return fmt.Errorf("permissions[%d]: expected %v, got %s", i, want, got)
			}
		}
		return nil
	}
}

// testAccRolePermissionsPreamble looks up the permission IDs of the "View Only"
// preset role, which every tenant has.
func testAccRolePermissionsPreamble() string {
	return `
data "doit_roles" "all" {}

locals {
  preset_permissions = one([
    for r in data.doit_roles.all.roles : r.permissions
    if r.type == "preset" && r.name == "View Only"
  ])
}
`
}

func testAccRoleConfig(name, description, permissions string) string {
	return testAccRolePermissionsPreamble() + fmt.Sprintf(`
resource "doit_role" "this" {
  name        = %[1]q
  description = %[2]q
  permissions = %[3]s
}

output "expected_permissions" {
  value = %[3]s
}
`, name, description, permissions)
}

func testAccRoleMinimalConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_role" "this" {
  name = %[1]q
}
`, name)
}

func testAccRoleRawConfig(body string) string {
	return fmt.Sprintf(`
resource "doit_role" "this" {
  %s
}
`, body)
}

func testAccRolePresetLookupConfig() string {
	return `
data "doit_roles" "presets" {}

output "preset_role_id" {
  value = one([
    for r in data.doit_roles.presets.roles : r.id
    if r.type == "preset" && r.name == "View Only"
  ])
}
`
}
