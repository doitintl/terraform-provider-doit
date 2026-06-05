package provider_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsNodeActivitiesDataSource_Basic(t *testing.T) {
	config := testAccCloudDiagramsNodeActivitiesDiscovery() + `
data "doit_cloud_diagrams_node_activities" "test" {
  ss_id   = local.first_layer_id
  node_id = local.first_node_id
}

# Verify that metadata is valid JSON by decoding it.
locals {
  activities      = tolist(data.doit_cloud_diagrams_node_activities.test.cloud_diagrams_node_activities)
  first_activity  = local.activities[0]
  metadata_parsed = local.first_activity.metadata != null ? jsondecode(local.first_activity.metadata) : null
}

output "metadata_has_keys" {
  value = local.metadata_parsed != null ? length(keys(local.metadata_parsed)) > 0 : false
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
					// Verify we got at least one activity record.
					resource.TestCheckResourceAttrWith("data.doit_cloud_diagrams_node_activities.test", "cloud_diagrams_node_activities.#", func(value string) error {
						n, err := strconv.Atoi(value)
						if err != nil {
							return fmt.Errorf("expected numeric count, got %q: %w", value, err)
						}
						if n == 0 {
							return fmt.Errorf("expected at least one activity, got 0")
						}
						return nil
					}),
					// Verify metadata is surfaced as valid JSON (jsondecode succeeded, has keys).
					resource.TestCheckOutput("metadata_has_keys", "true"),
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
`
}
