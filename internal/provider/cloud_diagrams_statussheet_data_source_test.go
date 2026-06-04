package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsStatussheetDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				// Chain: schemes → layer ID → discover component IDs → statussheet
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id       = local.first_layer_id
  node_ids = local.node_ids
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id       = local.first_layer_id
  node_ids = local.node_ids
}`,
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsStatussheetDataSource_MultipleComponentTypes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id        = local.first_layer_id
  node_ids  = local.node_ids
  group_ids = local.group_ids
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsStatussheetDataSource_WithProjection(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id       = local.first_layer_id
  node_ids = local.node_ids
  p        = "name icon color"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
		},
	})
}

// testAccCloudDiagramsStatussheetDiscovery chains from schemes to discover
// component IDs for the first available layer.
func testAccCloudDiagramsStatussheetDiscovery() string {
	return `
# Step 1: Get an overview of all diagrams.
data "doit_cloud_diagrams_schemes" "overview" {}

# Step 2: Pick the first diagram and its first layer.
locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.overview.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.overview.scheme[local.first_scheme_key].statussheet[0].ssid
}

# Step 3: Load the diagram with component data for that layer.
data "doit_cloud_diagrams_schemes" "with_components" {
  layer_ids = [local.first_layer_id]
}

# Step 4: Extract component IDs from the statussheet data.
locals {
  ss_data   = data.doit_cloud_diagrams_schemes.with_components.statussheet[local.first_layer_id]
  node_ids  = local.ss_data != null && local.ss_data.node != null ? keys(local.ss_data.node) : []
  group_ids = local.ss_data != null && local.ss_data.group != null ? keys(local.ss_data.group) : []
}
`
}
