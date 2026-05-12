package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccFolderDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-folder-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_folder.test", "id",
						"doit_folder.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_folder.test", "name",
						"doit_folder.test", "name"),
					resource.TestCheckResourceAttrPair(
						"data.doit_folder.test", "description",
						"doit_folder.test", "description"),
					resource.TestCheckResourceAttrPair(
						"data.doit_folder.test", "parent_folder_id",
						"doit_folder.test", "parent_folder_id"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccFolderDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccFolderDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "test" {
  name        = %q
  description = "test folder for data source"
}

data "doit_folder" "test" {
  id = doit_folder.test.id
}
`, name)
}

func TestAccFolderDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccFolderDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading folder|not found|404`),
			},
		},
	})
}

func testAccFolderDataSourceNotFoundConfig() string {
	return `
data "doit_folder" "notfound" {
  id = "nonexistent-folder-id"
}
`
}
