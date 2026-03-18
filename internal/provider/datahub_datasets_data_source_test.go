package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccDatahubDatasetsDataSource_Basic tests basic DataHub datasets list retrieval.
func TestAccDatahubDatasetsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDatahubDatasetsDataSourceBasicConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_datahub_datasets.test", "datasets.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccDatahubDatasetsDataSourceBasicConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccDatahubDatasetsDataSourceBasicConfig() string {
	return `
data "doit_datahub_datasets" "test" {}
`
}

// TestAccDatahubDatasetDataSource_Basic tests fetching a specific dataset by name.
// Creates a dataset via the resource, then reads it back via the data source.
func TestAccDatahubDatasetDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDatahubDatasetDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_datahub_dataset.test", "name", rName),
					resource.TestCheckResourceAttrSet("data.doit_datahub_dataset.test", "last_updated"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccDatahubDatasetDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccDatahubDatasetDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_datahub_dataset" "dep" {
  name        = %[1]q
  description = "Created by acceptance test for data source verification"
}

data "doit_datahub_dataset" "test" {
  name = doit_datahub_dataset.dep.name
}
`, name)
}
