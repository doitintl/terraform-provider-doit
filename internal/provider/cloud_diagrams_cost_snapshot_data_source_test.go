package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsCostSnapshotDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsCostSnapshotDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "diagram_id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "currency"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "total"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "time_range.start_date"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "time_range.end_date"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "time_range.interval"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_cost_snapshot.test", "start_date", "2026-01-01"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_cost_snapshot.test", "end_date", "2026-06-30"),
				),
			},
			{
				Config:   testAccCloudDiagramsCostSnapshotDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsCostSnapshotDataSource_WithInterval(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsCostSnapshotDataSourceConfigWithInterval("month"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "diagram_id"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_cost_snapshot.test", "interval", "month"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_cost_snapshot.test", "time_range.interval", "month"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsCostSnapshotDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudDiagramsCostSnapshotDataSourceConfigDirect("nonexistent-layer-id"),
				ExpectError: regexp.MustCompile(`(?i)403|404|Cloud Diagram Cost Snapshot`),
			},
		},
	})
}

func TestAccCloudDiagramsCostSnapshotDataSource_ListAttributes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsCostSnapshotDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "top_resources.#"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "by_service.#"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_cost_snapshot.test", "trend.#"),
				),
			},
		},
	})
}

func testAccCloudDiagramsCostSnapshotDataSourceConfig() string {
	return testAccCloudDiagramsSnapshotsLayerDiscovery() + `
data "doit_cloud_diagrams_cost_snapshot" "test" {
  id         = local.first_layer_id
  start_date = "2026-01-01"
  end_date   = "2026-06-30"
}
`
}

func testAccCloudDiagramsCostSnapshotDataSourceConfigWithInterval(interval string) string {
	return testAccCloudDiagramsSnapshotsLayerDiscovery() + `
data "doit_cloud_diagrams_cost_snapshot" "test" {
  id         = local.first_layer_id
  start_date = "2026-01-01"
  end_date   = "2026-06-30"
  interval   = "` + interval + `"
}
`
}

func testAccCloudDiagramsCostSnapshotDataSourceConfigDirect(layerID string) string {
	return `
data "doit_cloud_diagrams_cost_snapshot" "test" {
  id         = "` + layerID + `"
  start_date = "2026-01-01"
  end_date   = "2026-06-30"
}
`
}
