package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsNodeActivitiesDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsNodeActivitiesDiscovery() + `
data "doit_cloud_diagrams_node_activities" "test" {
  ss_id   = local.first_layer_id
  node_id = local.first_node_id
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_node_activities.test", "id"),
				),
			},
			{
				Config: testAccCloudDiagramsNodeActivitiesDiscovery() + `
data "doit_cloud_diagrams_node_activities" "test" {
  ss_id   = local.first_layer_id
  node_id = local.first_node_id
}`,
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsNodeActivitiesDataSource_WithLimit(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsNodeActivitiesDiscovery() + `
data "doit_cloud_diagrams_node_activities" "test" {
  ss_id   = local.first_layer_id
  node_id = local.first_node_id
  limit   = 5
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_node_activities.test", "id"),
				),
			},
		},
	})
}

// testAccCloudDiagramsNodeActivitiesDiscovery discovers a node ID from the
// first available diagram layer via the schemes data source.
func testAccCloudDiagramsNodeActivitiesDiscovery() string {
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

# Step 4: Extract the first node ID from the statussheet data.
locals {
  ss_data       = data.doit_cloud_diagrams_schemes.with_components.statussheet[local.first_layer_id]
  first_node_id = keys(local.ss_data.node)[0]
}
`
}
