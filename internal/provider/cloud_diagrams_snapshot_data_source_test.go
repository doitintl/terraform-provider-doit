package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsSnapshotDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Chain: schemes → first layer → snapshots list → first snapshot
				Config: testAccCloudDiagramsSnapshotDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshot.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshot.test", "snapshot_id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshot.test", "created_at"),
				),
			},
			{
				Config:   testAccCloudDiagramsSnapshotDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsSnapshotDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudDiagramsSnapshotDataSourceConfigDirect("nonexistent-layer-id", "nonexistent-snapshot-id"),
				ExpectError: regexp.MustCompile(`(?i)not found|403|404|Cloud Diagram Snapshot`),
			},
		},
	})
}

func testAccCloudDiagramsSnapshotDataSourceConfig() string {
	return testAccCloudDiagramsSnapshotsLayerDiscovery() + `
# List snapshots for the layer (need at least 1).
data "doit_cloud_diagrams_snapshots" "list" {
  id    = local.first_layer_id
  limit = 1
}

locals {
  first_snapshot = tolist(data.doit_cloud_diagrams_snapshots.list.cloud_diagrams_snapshots)[0]
}

# Get the single snapshot.
data "doit_cloud_diagrams_snapshot" "test" {
  id          = local.first_layer_id
  snapshot_id = local.first_snapshot._id
}
`
}

func testAccCloudDiagramsSnapshotDataSourceConfigDirect(layerID, snapshotID string) string {
	return `
data "doit_cloud_diagrams_snapshot" "test" {
  id          = "` + layerID + `"
  snapshot_id = "` + snapshotID + `"
}
`
}
