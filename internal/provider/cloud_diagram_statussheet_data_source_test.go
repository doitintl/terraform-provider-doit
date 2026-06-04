package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramStatussheetDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Chain: schemes overview → first scheme → first layer → statussheet
				Config: testAccCloudDiagramStatussheetDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_statussheet.test", "id"),
				),
			},
			{
				Config:   testAccCloudDiagramStatussheetDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramStatussheetDataSource_WithProjection(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramStatussheetDataSourceConfigWithProjection(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_statussheet.test", "id"),
				),
			},
		},
	})
}

func testAccCloudDiagramStatussheetDataSourceConfig() string {
	return `
# Step 1: Discover all diagrams (scheme overview).
data "doit_cloud_diagram_schemes" "overview" {}

# Step 2: Pick the first diagram and load its layers.
locals {
  scheme_ids = keys(data.doit_cloud_diagram_schemes.overview.scheme)
  first_id   = length(local.scheme_ids) > 0 ? [local.scheme_ids[0]] : []
}

data "doit_cloud_diagram_schemes" "detail" {
  scheme_ids = local.first_id
}

# Step 3: Pick the first layer from the first diagram.
locals {
  first_scheme = [for k, v in data.doit_cloud_diagram_schemes.detail.scheme : v][0]
  first_layer  = local.first_scheme.statussheet[0]
}

# Step 4: Get all components of that layer.
data "doit_cloud_diagram_statussheet" "test" {
  id = local.first_layer.ssid
}
`
}

func testAccCloudDiagramStatussheetDataSourceConfigWithProjection() string {
	return `
data "doit_cloud_diagram_schemes" "overview" {}

locals {
  scheme_ids = keys(data.doit_cloud_diagram_schemes.overview.scheme)
  first_id   = length(local.scheme_ids) > 0 ? [local.scheme_ids[0]] : []
}

data "doit_cloud_diagram_schemes" "detail" {
  scheme_ids = local.first_id
}

locals {
  first_scheme = [for k, v in data.doit_cloud_diagram_schemes.detail.scheme : v][0]
  first_layer  = local.first_scheme.statussheet[0]
}

data "doit_cloud_diagram_statussheet" "test" {
  id = local.first_layer.ssid
  p  = "name icon color"
}
`
}
