package provider_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccFolder(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create folder at root with explicit parent_folder_id
			{
				Config: testAccFolderBasic(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_folder.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 2: Update folder (change name and add description)
			{
				Config: testAccFolderUpdated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_folder.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName+"-updated")),
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated description")),
				},
			},
			// Step 3: Drift check — re-apply same updated config, expect no changes.
			{
				Config: testAccFolderUpdated(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccFolder_Import(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderBasic(rName),
			},
			{
				ResourceName:      "doit_folder.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Drift check — re-apply config after import, expect no changes.
			{
				Config: testAccFolderBasic(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccFolder_OmittedOptionalComputed(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with only required fields (omit description and parent_folder_id)
			{
				Config: testAccFolderMinimal(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					// parent_folder_id should be resolved to "root"
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 2: Drift check — re-apply, expect no changes.
			{
				Config: testAccFolderMinimal(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccFolder_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
func TestAccFolder_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccFolderBasic(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("doit_folder.this", "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("resource ID is empty")
						}
						resourceId = value
						return nil
					}),
				),
			},
			// Step 2: Delete the resource via API, then verify Terraform detects the drift
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteFolderWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete folder via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccFolderBasic(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}

func TestAccFolder_NestedFolder(t *testing.T) {
	rParent := acctest.RandomWithPrefix("tf-acc-parent")
	rChild := acctest.RandomWithPrefix("tf-acc-child")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create parent + child folder
			{
				Config: testAccFolderNested(rParent, rChild),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.parent",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccFolderNested(rParent, rChild),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccFolder_EmptyDescription verifies that setting description = "" is
// preserved as a real value and does not cause drift.
func TestAccFolder_EmptyDescription(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with explicit empty description
			{
				Config: testAccFolderEmptyDescription(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("")),
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 2: Drift check — empty string should not cause perpetual diff
			{
				Config: testAccFolderEmptyDescription(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccFolder_EmptyParentFolderIdRejected verifies that setting
// parent_folder_id = "" is rejected at plan time. The API normalizes ""
// to "root", so allowing it would cause perpetual plan drift.
func TestAccFolder_EmptyParentFolderIdRejected(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccFolderEmptyParentFolderId(rName),
				ExpectError: regexp.MustCompile(`(?i)string length must be at least 1`),
			},
		},
	})
}

// TestAccFolder_MoveFolder verifies that a child folder can be moved
// from one parent to another by changing parent_folder_id.
func TestAccFolder_MoveFolder(t *testing.T) {
	rParentA := acctest.RandomWithPrefix("tf-acc-parent-a")
	rParentB := acctest.RandomWithPrefix("tf-acc-parent-b")
	rChild := acctest.RandomWithPrefix("tf-acc-child")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create two parents + child under parent_a
			{
				Config: testAccFolderMove(rParentA, rParentB, rChild, "doit_folder.parent_a.id"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.parent_a",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
					statecheck.ExpectKnownValue(
						"doit_folder.parent_b",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 2: Drift check before move
			{
				Config: testAccFolderMove(rParentA, rParentB, rChild, "doit_folder.parent_a.id"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Move child from parent_a to parent_b
			{
				Config: testAccFolderMove(rParentA, rParentB, rChild, "doit_folder.parent_b.id"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_folder.child",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Step 4: Drift check after move
			{
				Config: testAccFolderMove(rParentA, rParentB, rChild, "doit_folder.parent_b.id"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Move child to root
			{
				Config: testAccFolderMoveToRoot(rParentA, rParentB, rChild),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_folder.child",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.child",
						tfjsonpath.New("parent_folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 6: Drift check after move to root
			{
				Config: testAccFolderMoveToRoot(rParentA, rParentB, rChild),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccFolder_UpdateNamePreservesParent verifies that updating an unrelated
// field (name) on a nested folder does NOT move it to root. This disproves the
// concern that toUpdateRequest() defaults parent_folder_id to "root" when the
// user only changes another field.
// Ref: https://github.com/doitintl/terraform-provider-doit/pull/199#discussion_r3302936367
func TestAccFolder_UpdateNamePreservesParent(t *testing.T) {
	rParent := acctest.RandomWithPrefix("tf-acc-parent")
	rChild := acctest.RandomWithPrefix("tf-acc-child")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create parent + child nested under parent
			{
				Config: testAccFolderNested(rParent, rChild),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.child",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rChild)),
				},
			},
			// Step 2: Update only the child's name — parent_folder_id stays the same
			{
				Config: testAccFolderNested(rParent, rChild+"-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_folder.child",
							plancheck.ResourceActionUpdate,
						),
						// Parent folder should NOT be planned for update
						plancheck.ExpectResourceAction(
							"doit_folder.parent",
							plancheck.ResourceActionNoop,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.child",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rChild+"-renamed")),
					// The key assertion: parent_folder_id must NOT have changed to "root"
					statecheck.CompareValuePairs(
						"doit_folder.parent",
						tfjsonpath.New("id"),
						"doit_folder.child",
						tfjsonpath.New("parent_folder_id"),
						compare.ValuesSame(),
					),
				},
			},
			// Step 3: Drift check — no changes expected
			{
				Config: testAccFolderNested(rParent, rChild+"-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// --- Config helpers ---

func testAccFolderMinimal(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "this" {
  name = %q
}
`, name)
}

func testAccFolderBasic(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "this" {
  name             = %q
  parent_folder_id = "root"
}
`, name)
}

func testAccFolderUpdated(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "this" {
  name             = "%s-updated"
  description      = "Updated description"
  parent_folder_id = "root"
}
`, name)
}

func testAccFolderEmptyDescription(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "this" {
  name             = %q
  description      = ""
  parent_folder_id = "root"
}
`, name)
}

func testAccFolderEmptyParentFolderId(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "this" {
  name             = %q
  parent_folder_id = ""
}
`, name)
}

func testAccFolderNested(parentName, childName string) string {
	return fmt.Sprintf(`
resource "doit_folder" "parent" {
  name             = %q
  parent_folder_id = "root"
}

resource "doit_folder" "child" {
  name             = %q
  parent_folder_id = doit_folder.parent.id
}
`, parentName, childName)
}

func testAccFolderMove(parentA, parentB, child, childParentRef string) string {
	return fmt.Sprintf(`
resource "doit_folder" "parent_a" {
  name             = %q
  parent_folder_id = "root"
}

resource "doit_folder" "parent_b" {
  name             = %q
  parent_folder_id = "root"
}

resource "doit_folder" "child" {
  name             = %q
  parent_folder_id = %s
}
`, parentA, parentB, child, childParentRef)
}

func testAccFolderMoveToRoot(parentA, parentB, child string) string {
	return fmt.Sprintf(`
resource "doit_folder" "parent_a" {
  name             = %q
  parent_folder_id = "root"
}

resource "doit_folder" "parent_b" {
  name             = %q
  parent_folder_id = "root"
}

resource "doit_folder" "child" {
  name             = %q
  parent_folder_id = "root"
}
`, parentA, parentB, child)
}

// TestAccFolder_ClearDescription tests that setting a description and then
// removing it from config results in no drift. This is a diagnostic test
// for https://github.com/doitintl/terraform-provider-doit/issues/233.
// If the clearing drift check fails, it confirms that the Optional+Computed
// attribute cannot be cleared due to the framework bug.
func TestAccFolder_ClearDescription(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with description set
			{
				Config: testAccFolderWithDescription(rName, "initial description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_folder.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("initial description")),
				},
			},
			// Step 2: Drift check — same config, expect no changes
			{
				Config: testAccFolderWithDescription(rName, "initial description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear description by omitting it from config
			{
				Config: testAccFolderMinimal(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// If issue #233 is real, this will be an empty plan
						// (the old value "sticks") instead of showing a change.
						// If the attribute CAN be cleared, this should be non-empty.
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 4: Drift check — cleared config should produce no drift
			{
				Config: testAccFolderMinimal(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccFolder_ClearParentFolderId tests that setting parent_folder_id and then
// removing it from config results in no drift. The API defaults to "root".
func TestAccFolder_ClearParentFolderId(t *testing.T) {
	rParent := acctest.RandomWithPrefix("tf-acc-parent")
	rChild := acctest.RandomWithPrefix("tf-acc-child")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create child under parent (parent_folder_id is set)
			{
				Config: testAccFolderNested(rParent, rChild),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(
						"doit_folder.parent",
						tfjsonpath.New("id"),
						"doit_folder.child",
						tfjsonpath.New("parent_folder_id"),
						compare.ValuesSame(),
					),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccFolderNested(rParent, rChild),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear parent_folder_id by omitting it — should move to root
			{
				Config: testAccFolderClearParent(rParent, rChild),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// If issue #233 is real for parent_folder_id, this will
						// be empty (old parent value sticks) instead of showing
						// an update to move the folder to root.
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 4: Drift check — should be stable at root
			{
				Config: testAccFolderClearParent(rParent, rChild),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccFolderWithDescription(name, description string) string {
	return fmt.Sprintf(`
resource "doit_folder" "this" {
  name        = %q
  description = %q
}
`, name, description)
}

func testAccFolderClearParent(parentName, childName string) string {
	return fmt.Sprintf(`
resource "doit_folder" "parent" {
  name             = %q
  parent_folder_id = "root"
}

resource "doit_folder" "child" {
  name = %q
}
`, parentName, childName)
}
