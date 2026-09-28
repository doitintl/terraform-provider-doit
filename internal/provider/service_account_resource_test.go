package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccServiceAccount_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccServiceAccountConfig(rName, "Test service account", `["budgetsReadOnly"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_service_account.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test service account"),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsReadOnly"),
						}),
					),
				},
			},
			// Step 2: Drift check — re-apply same config, expect empty plan
			{
				Config: testAccServiceAccountConfig(rName, "Test service account", `["budgetsReadOnly"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Update
			{
				Config: testAccServiceAccountConfig(rName+"-upd", "Updated description", `["budgetsReadOnly", "budgetsManager"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_service_account.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName+"-upd"),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated description"),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsReadOnly"),
							knownvalue.StringExact("budgetsManager"),
						}),
					),
				},
			},
			// Step 4: Drift check
			{
				Config: testAccServiceAccountConfig(rName+"-upd", "Updated description", `["budgetsReadOnly", "budgetsManager"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServiceAccount_ClearableLifecycle(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa-clear")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with populated clearable attributes
			{
				Config: testAccServiceAccountConfig(rName, "Initial description", `["budgetsReadOnly", "budgetsManager"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Initial description"),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsReadOnly"),
							knownvalue.StringExact("budgetsManager"),
						}),
					),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccServiceAccountConfig(rName, "Initial description", `["budgetsReadOnly", "budgetsManager"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear both description and permissions (omit them)
			{
				Config: testAccServiceAccountMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_service_account.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{}),
					),
				},
			},
			// Step 4: Drift check — verify empty config produces no drift against cleared state
			{
				Config: testAccServiceAccountMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Restore description and permissions
			{
				Config: testAccServiceAccountConfig(rName, "Restored description", `["budgetsReadOnly"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_service_account.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Restored description"),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsReadOnly"),
						}),
					),
				},
			},
			// Step 6: Drift check
			{
				Config: testAccServiceAccountConfig(rName, "Restored description", `["budgetsReadOnly"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServiceAccount_OmittedOptional(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa-omit")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create omitting all optional fields
			{
				Config: testAccServiceAccountMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{}),
					),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccServiceAccountMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServiceAccount_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa-import")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccServiceAccountConfig(rName, "Import test account", `["budgetsReadOnly", "budgetsManager"]`),
			},
			// Step 2: Import
			{
				ResourceName:            "doit_service_account.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"etag"},
			},
			// Step 3: Drift check after import
			{
				Config: testAccServiceAccountConfig(rName, "Import test account", `["budgetsReadOnly", "budgetsManager"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccServiceAccountConfig(name, description, permissionsJSON string) string {
	return fmt.Sprintf(`
resource "doit_service_account" "this" {
  name        = %q
  description = %q
  permissions = %s
}
`, name, description, permissionsJSON)
}

func testAccServiceAccountMinimalConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_service_account" "this" {
  name = %q
}
`, name)
}
