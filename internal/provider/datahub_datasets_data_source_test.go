package provider_test

import (
	"fmt"
	"os"
	"testing"

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
func TestAccDatahubDatasetDataSource_Basic(t *testing.T) {
	datasetName := os.Getenv("TEST_DATASET_NAME")
	if datasetName == "" {
		t.Skip("TEST_DATASET_NAME not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDatahubDatasetDataSourceConfig(datasetName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_datahub_dataset.test", "name", datasetName),
					resource.TestCheckResourceAttrSet("data.doit_datahub_dataset.test", "last_updated"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccDatahubDatasetDataSourceConfig(datasetName),
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
data "doit_datahub_dataset" "test" {
  name = %[1]q
}
`, name)
}
