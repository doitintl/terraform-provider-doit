package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsRelationshipsDataSource_Basic(t *testing.T) {
	config := testAccCloudDiagramsRelationshipsDiscovery() + `
data "doit_cloud_diagrams_relationships" "test" {
  id        = local.first_layer_id
  rid       = local.first_node_id
  direction = "both"
  depth     = "direct"
  kind      = "both"
}`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_relationships.test", "direction", "both"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_relationships.test", "depth", "direct"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_relationships.test", "kind", "both"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_relationships.test", "anchor.id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_relationships.test", "anchor.name"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_relationships.test", "anchor.type"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_relationships.test", "truncated"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsRelationshipsDataSource_EdgesOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsRelationshipsDiscovery() + `
data "doit_cloud_diagrams_relationships" "edges" {
  id        = local.first_layer_id
  rid       = local.first_node_id
  direction = "downstream"
  depth     = "transitive"
  kind      = "edges"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_relationships.edges", "direction", "downstream"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_relationships.edges", "depth", "transitive"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_relationships.edges", "kind", "edges"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_relationships.edges", "anchor.name"),
				),
			},
		},
	})
}

// testAccCloudDiagramsRelationshipsDiscovery discovers a node ID using
// the search data source, which returns nodes that exist in diagrams.
func testAccCloudDiagramsRelationshipsDiscovery() string {
	return `
# Step 1: Search for components using a common query.
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
    error_message = "Search for 'peer' returned no node components; cannot run relationships tests."
  }
}
`
}
