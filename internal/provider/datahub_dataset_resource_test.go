package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccDatahubDatasetResource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-dataset")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create without display_name (minimal config)
			{
				Config: testAccDatahubDatasetResource(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttrSet("doit_datahub_dataset.test", "last_updated"),
					resource.TestCheckNoResourceAttr("doit_datahub_dataset.test", "display_name"),
				),
			},
			// Step 2: Drift check (minimal config)
			{
				Config: testAccDatahubDatasetResource(rName, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Set display_name, description, logo_name
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "Updated description", "aws", "Initial Display"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "description", "Updated description"),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "display_name", "Initial Display"),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "logo_name", "aws"),
				),
			},
			// Step 4: Drift check
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "Updated description", "aws", "Initial Display"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 5: Update display_name and logo_name
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "Updated description", "gcp", "Updated Display"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "description", "Updated description"),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "display_name", "Updated Display"),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "logo_name", "gcp"),
				),
			},
			// Step 6: Drift check
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "Updated description", "gcp", "Updated Display"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 7: Clear display_name by omitting it
			{
				Config: testAccDatahubDatasetResourceFull(rName, "Updated description", "gcp"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "description", "Updated description"),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "logo_name", "gcp"),
				),
			},
			// Step 8: Drift check on cleared display_name
			{
				Config: testAccDatahubDatasetResourceFull(rName, "Updated description", "gcp"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 9: Restore display_name
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "Updated description", "gcp", "Restored Display"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "display_name", "Restored Display"),
				),
			},
			// Step 10: Drift check
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "Updated description", "gcp", "Restored Display"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 11: Import verification with display_name present
			{
				ResourceName:                         "doit_datahub_dataset.test",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccDatahubDatasetResource_NameChangeTriggersReplace(t *testing.T) {
	originalName := acctest.RandomWithPrefix("tf-acc-dataset")
	newName := acctest.RandomWithPrefix("tf-acc-dataset")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDatahubDatasetResource(originalName, "original"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", originalName),
				),
			},
			{
				Config: testAccDatahubDatasetResource(newName, "original"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_datahub_dataset.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", newName),
				),
			},
		},
	})
}

func TestAccDatahubDatasetResource_Disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-dataset")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDatahubDatasetResource(rName, "will be deleted"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
				),
			},
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteDatahubDatasetWithResponse(t.Context(), rName)
					if err != nil {
						t.Fatalf("Failed to delete dataset via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccDatahubDatasetResource(rName, "will be deleted"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccDatahubDatasetResource(name, description string) string {
	return testAccDatahubDatasetResourceComplete(name, description, "", "")
}

func testAccDatahubDatasetResourceFull(name, description, logoName string) string {
	return testAccDatahubDatasetResourceComplete(name, description, logoName, "")
}

func testAccDatahubDatasetResourceComplete(name, description, logoName, displayName string) string {
	var body string
	if description != "" {
		body += fmt.Sprintf("  description  = %q\n", description)
	}
	if displayName != "" {
		body += fmt.Sprintf("  display_name = %q\n", displayName)
	}
	if logoName != "" {
		body += fmt.Sprintf("  logo_name    = %q\n", logoName)
	}

	return fmt.Sprintf(`
resource "doit_datahub_dataset" "test" {
  name = %[1]q
%[2]s}
`, name, body)
}

// TestAccDatahubDatasetResource_ClearDescription tests that setting a description
// and then removing it from config results in no drift. This is a diagnostic test
// for https://github.com/doitintl/terraform-provider-doit/issues/233.
func TestAccDatahubDatasetResource_ClearDescription(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-dataset")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with description set
			{
				Config: testAccDatahubDatasetResource(rName, "initial description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "description", "initial description"),
				),
			},
			// Step 2: Drift check
			{
				Config: testAccDatahubDatasetResource(rName, "initial description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear description by omitting it
			{
				Config: testAccDatahubDatasetResource(rName, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 4: Drift check — cleared value should produce no drift
			{
				Config: testAccDatahubDatasetResource(rName, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccDatahubDatasetResource_ClearLogoName tests that setting logo_name
// and then removing it from config clears the value with no drift (Category A).
func TestAccDatahubDatasetResource_ClearLogoName(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-dataset")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with logo_name set
			{
				Config: testAccDatahubDatasetResourceFull(rName, "some description", "aws"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "logo_name", "aws"),
				),
			},
			// Step 2: Drift check
			{
				Config: testAccDatahubDatasetResourceFull(rName, "some description", "aws"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear logo_name by omitting it
			{
				Config: testAccDatahubDatasetResource(rName, "some description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 4: Drift check — cleared value should produce no drift
			{
				Config: testAccDatahubDatasetResource(rName, "some description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccDatahubDatasetResource_ClearDisplayName tests that setting display_name
// and then removing it from config clears the value with no drift (Category A).
func TestAccDatahubDatasetResource_ClearDisplayName(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-dataset")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with display_name set
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "some description", "aws", "Custom Display Name"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "display_name", "Custom Display Name"),
				),
			},
			// Step 2: Drift check
			{
				Config: testAccDatahubDatasetResourceComplete(rName, "some description", "aws", "Custom Display Name"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear display_name by omitting it
			{
				Config: testAccDatahubDatasetResourceFull(rName, "some description", "aws"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			// Step 4: Drift check — cleared value should produce no drift
			{
				Config: testAccDatahubDatasetResourceFull(rName, "some description", "aws"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
