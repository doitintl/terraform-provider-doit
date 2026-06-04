package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsStatussheetDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Chain: schemes overview → first scheme → first layer → statussheet
				Config: testAccCloudDiagramsStatussheetDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
			{
				Config:   testAccCloudDiagramsStatussheetDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsStatussheetDataSource_WithProjection(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsStatussheetDataSourceConfigWithProjection(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
		},
	})
}

func testAccCloudDiagramsStatussheetDataSourceConfig() string {
	return `
# Step 1: Discover all diagrams (scheme overview).
data "doit_cloud_diagrams_schemes" "overview" {}

# Step 2: Pick the first diagram and load its layers.
locals {
  scheme_ids = keys(data.doit_cloud_diagrams_schemes.overview.scheme)
  first_id   = length(local.scheme_ids) > 0 ? [local.scheme_ids[0]] : []
}

data "doit_cloud_diagrams_schemes" "detail" {
  scheme_ids = local.first_id
}

# Step 3: Pick the first layer from the first diagram.
locals {
  first_scheme = [for k, v in data.doit_cloud_diagrams_schemes.detail.scheme : v][0]
  first_layer  = local.first_scheme.statussheet[0]
}

# Step 4: Get all components of that layer.
data "doit_cloud_diagrams_statussheet" "test" {
  id = local.first_layer.ssid
}
`
}

func testAccCloudDiagramsStatussheetDataSourceConfigWithProjection() string {
	return `
data "doit_cloud_diagrams_schemes" "overview" {}

locals {
  scheme_ids = keys(data.doit_cloud_diagrams_schemes.overview.scheme)
  first_id   = length(local.scheme_ids) > 0 ? [local.scheme_ids[0]] : []
}

data "doit_cloud_diagrams_schemes" "detail" {
  scheme_ids = local.first_id
}

locals {
  first_scheme = [for k, v in data.doit_cloud_diagrams_schemes.detail.scheme : v][0]
  first_layer  = local.first_scheme.statussheet[0]
}

data "doit_cloud_diagrams_statussheet" "test" {
  id = local.first_layer.ssid
  p  = "name icon color"
}
`
}
