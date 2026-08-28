package provider_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccDatahubDatasetsDataSource_Basic tests basic DataHub datasets list retrieval.
// Creates a dataset with a logo via the resource, then verifies the plural data source includes it.
func TestAccDatahubDatasetsDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-ds-list")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDatahubDatasetsDataSourceBasicConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_datahub_datasets.test", "datasets.#"),
					testCheckDatahubDatasetsContains("data.doit_datahub_datasets.test", rName, "aws"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccDatahubDatasetsDataSourceBasicConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testCheckDatahubDatasetsContains(dataSourceName, datasetName, expectedLogoName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found: %s", dataSourceName)
		}

		for key, val := range rs.Primary.Attributes {
			if strings.HasPrefix(key, "datasets.") && strings.HasSuffix(key, ".name") && val == datasetName {
				prefix := strings.TrimSuffix(key, ".name")
				logoKey := prefix + ".logo_name"
				actualLogo, ok := rs.Primary.Attributes[logoKey]
				if !ok {
					return fmt.Errorf("attribute %s not found in state", logoKey)
				}
				if actualLogo != expectedLogoName {
					return fmt.Errorf("expected %s to be %q, got %q", logoKey, expectedLogoName, actualLogo)
				}
				return nil
			}
		}
		return fmt.Errorf("dataset %q not found in %s attributes", datasetName, dataSourceName)
	}
}

func testAccDatahubDatasetsDataSourceBasicConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_datahub_dataset" "dep" {
  name        = %[1]q
  description = "Created by acceptance test for datasets list verification"
  logo_name   = "aws"
}

data "doit_datahub_datasets" "test" {
  depends_on = [doit_datahub_dataset.dep]
}
`, name)
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
					resource.TestCheckResourceAttr("data.doit_datahub_dataset.test", "description", "Created by acceptance test for data source verification"),
					resource.TestCheckResourceAttr("data.doit_datahub_dataset.test", "logo_name", "aws"),
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
  logo_name   = "aws"
}

data "doit_datahub_dataset" "test" {
  name = doit_datahub_dataset.dep.name
}
`, name)
}
