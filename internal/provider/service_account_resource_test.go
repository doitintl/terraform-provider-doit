package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// testAccCheckServiceAccountEtagCurrent compares the stored etag with the one
// the API returns, so a stale etag left by an update fails the step.
func testAccCheckServiceAccountEtagCurrent(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["doit_service_account.this"]
		if !ok {
			return fmt.Errorf("doit_service_account.this not found in state")
		}
		resp, err := getAPIClient(t).GetServiceAccountWithResponse(t.Context(), rs.Primary.ID)
		if err != nil || resp.JSON200 == nil {
			return fmt.Errorf("reading service account: %v", err)
		}
		etag, _ := resp.JSON200.Etag.Get()
		if got := rs.Primary.Attributes["etag"]; etag == "" || got != etag {
			return fmt.Errorf("etag in state %q, API returns %q", got, etag)
		}
		return nil
	}
}

// An update whose only change comes from a clearing plan modifier (Terraform
// proposed the stored value) must still plan etag and update_time unknown, or
// state keeps the old etag and the next update without refresh fails with 412.
func TestAccServiceAccount_ClearOnlyUpdateRefreshesEtag(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa-etag")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: testAccServiceAccountConfig(rName, "Initial description", `["budgetsReadOnly"]`)},
			{
				Config: testAccServiceAccountMinimalConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_service_account.this", plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue("doit_service_account.this", tfjsonpath.New("etag")),
						plancheck.ExpectUnknownValue("doit_service_account.this", tfjsonpath.New("update_time")),
					},
				},
				Check: testAccCheckServiceAccountEtagCurrent(t),
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
			// Configuration generated for the import (an import block with
			// -generate-config-out) is valid and plans no change.
			{
				ResourceName:    "doit_service_account.this",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
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

func TestAccServiceAccount_ExplicitEmptyPermissions(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa-empty")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create explicitly configuring permissions = []
			{
				Config: testAccServiceAccountConfig(rName, "Test empty permissions", "[]"),
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
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{}),
					),
				},
			},
			// Step 2: Drift check — re-apply same config, expect empty plan
			{
				Config: testAccServiceAccountConfig(rName, "Test empty permissions", "[]"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServiceAccount_PermissionsReordering(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sa-reorder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with permissions in order A: ["budgetsReadOnly", "budgetsManager"]
			{
				Config: testAccServiceAccountConfig(rName, "Test permissions ordering", `["budgetsReadOnly", "budgetsManager"]`),
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
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsReadOnly"),
							knownvalue.StringExact("budgetsManager"),
						}),
					),
				},
			},
			// Step 2: Drift check for order A
			{
				Config: testAccServiceAccountConfig(rName, "Test permissions ordering", `["budgetsReadOnly", "budgetsManager"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Update with permissions in reversed order B: ["budgetsManager", "budgetsReadOnly"]
			{
				Config: testAccServiceAccountConfig(rName, "Test permissions ordering", `["budgetsManager", "budgetsReadOnly"]`),
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
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsManager"),
							knownvalue.StringExact("budgetsReadOnly"),
						}),
					),
				},
			},
			// Step 4: Drift check for order B
			{
				Config: testAccServiceAccountConfig(rName, "Test permissions ordering", `["budgetsManager", "budgetsReadOnly"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_service_account.this",
						tfjsonpath.New("permissions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("budgetsManager"),
							knownvalue.StringExact("budgetsReadOnly"),
						}),
					),
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
