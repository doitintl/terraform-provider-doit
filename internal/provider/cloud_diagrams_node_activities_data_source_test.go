package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsNodeActivitiesDataSource_Basic(t *testing.T) {
	config := testAccCloudDiagramsNodeActivitiesDiscovery() + `
data "doit_cloud_diagrams_node_activities" "test" {
  ss_id   = local.first_layer_id
  node_id = local.first_node_id
  limit   = 1
}

# Verify that metadata, when present, is valid JSON.
# Use limit=1 so the set has exactly one element, avoiding ordering flakiness.
locals {
  activities     = tolist(data.doit_cloud_diagrams_node_activities.test.cloud_diagrams_node_activities)
  first_activity = local.activities[0]
}

output "metadata_valid" {
  # metadata is either null (some activity types omit it) or a valid JSON object.
  value = local.first_activity.metadata == null ? true : can(jsondecode(local.first_activity.metadata))
}`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_node_activities.test", "id"),
					// Verify we got exactly one activity record (limit=1).
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_node_activities.test", "cloud_diagrams_node_activities.#", "1"),
					// Verify metadata is either null or decodable JSON.
					resource.TestCheckOutput("metadata_valid", "true"),
				),
			},
			{
				Config:   config,
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

// testAccCloudDiagramsNodeActivitiesDiscovery discovers a node ID using
// the search data source, which returns nodes that exist in diagrams and
// therefore have at least a NODE_CREATE activity record.
func testAccCloudDiagramsNodeActivitiesDiscovery() string {
	return `
# Step 1: Search for components using the test resource name.
data "doit_cloud_diagrams_search" "lookup" {
  query = "peer"
}

# Step 2: Pick the first node-type component from search results.
locals {
  nodes          = [for c in data.doit_cloud_diagrams_search.lookup.component : c if c.type == "node"]
  first_layer_id = local.nodes[0].ss_id
  first_node_id  = local.nodes[0]._id
}

# Guard: fail with a clear message if the search returned no nodes.
check "has_nodes" {
  assert {
    condition     = length(local.nodes) > 0
    error_message = "Search for 'peer' returned no node components; cannot run node_activities tests."
  }
}
`
}
