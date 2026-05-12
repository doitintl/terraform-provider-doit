package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccFoldersDataSource_AutoPagination tests that without max_results, all folders are fetched.
func TestAccFoldersDataSource_AutoPagination(t *testing.T) {
	// Create a folder so the list is guaranteed non-empty, then list all.
	rName := acctest.RandomWithPrefix("tf-acc-folders-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccFoldersDataSourceAutoConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_folders.all", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_folders.all", "page_token"),
				),
			},
			// Drift verification
			{
				Config: testAccFoldersDataSourceAutoConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccFoldersDataSourceAutoConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_folder" "seed" {
  name = %q
}

data "doit_folders" "all" {
  depends_on = [doit_folder.seed]
}
`, name)
}

// TestAccFoldersDataSource_MaxResults tests that setting max_results limits results.
func TestAccFoldersDataSource_MaxResults(t *testing.T) {
	rName1 := acctest.RandomWithPrefix("tf-acc-folders-ds-a")
	rName2 := acctest.RandomWithPrefix("tf-acc-folders-ds-b")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccFoldersDataSourceMaxResultsConfig(rName1, rName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_folders.limited", "folders.#", "1"),
				),
			},
		},
	})
}

func testAccFoldersDataSourceMaxResultsConfig(name1, name2 string) string {
	return fmt.Sprintf(`
resource "doit_folder" "a" {
  name = %q
}

resource "doit_folder" "b" {
  name = %q
}

data "doit_folders" "limited" {
  max_results = "1"
  depends_on  = [doit_folder.a, doit_folder.b]
}
`, name1, name2)
}
