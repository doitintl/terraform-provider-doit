package provider_test

import (
	"context"
	"fmt"
	"testing"

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
			// Step 1: Create folder at root (omit parent_folder_id)
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
