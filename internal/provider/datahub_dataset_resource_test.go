package provider_test

import (
	"context"
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
			{
				Config: testAccDatahubDatasetResource(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttrSet("doit_datahub_dataset.test", "last_updated"),
				),
			},
			{
				Config: testAccDatahubDatasetResource(rName, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccDatahubDatasetResource(rName, "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("doit_datahub_dataset.test", "description", "Updated description"),
				),
			},
			{
				Config: testAccDatahubDatasetResource(rName, "Updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
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
					resp, err := client.DeleteDatahubDatasetWithResponse(context.Background(), rName)
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
	if description != "" {
		return fmt.Sprintf(`
resource "doit_datahub_dataset" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
	}

	return fmt.Sprintf(`
resource "doit_datahub_dataset" "test" {
  name = %[1]q
}
`, name)
}
