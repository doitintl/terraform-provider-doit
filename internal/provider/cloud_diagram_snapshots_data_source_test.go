package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramSnapshotsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramSnapshotsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_snapshots.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_snapshots.test", "cloud_diagram_snapshots.#"),
				),
			},
			{
				Config:   testAccCloudDiagramSnapshotsDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramSnapshotsDataSource_WithOptions(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramSnapshotsDataSourceConfigWithOptions(2, 0, "-createdAt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_snapshots.test", "id"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "limit", "2"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "offset", "0"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "sort", "-createdAt"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_snapshots.test", "cloud_diagram_snapshots.#"),
				),
			},
			{
				Config:   testAccCloudDiagramSnapshotsDataSourceConfigWithOptions(2, 0, "-createdAt"),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramSnapshotsDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudDiagramSnapshotsDataSourceConfigDirect("nonexistent-layer-id-12345"),
				ExpectError: regexp.MustCompile(`(?i)not found|404|Cloud Diagram layer`),
			},
		},
	})
}

// testAccCloudDiagramSnapshotsLayerDiscovery returns the HCL block that
// discovers the first available layer ID from doit_cloud_diagram_schemes.
func testAccCloudDiagramSnapshotsLayerDiscovery() string {
	return `
# Discover the first available layer ID from the schemes endpoint.
data "doit_cloud_diagram_schemes" "discovery" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagram_schemes.discovery.scheme)[0]
  first_layer_id   = data.doit_cloud_diagram_schemes.discovery.scheme[local.first_scheme_key].statussheet[0].ssid
}
`
}

func testAccCloudDiagramSnapshotsDataSourceConfig() string {
	return testAccCloudDiagramSnapshotsLayerDiscovery() + `
data "doit_cloud_diagram_snapshots" "test" {
  id = local.first_layer_id
}
`
}

func testAccCloudDiagramSnapshotsDataSourceConfigWithOptions(limit, offset int, sort string) string {
	return testAccCloudDiagramSnapshotsLayerDiscovery() + fmt.Sprintf(`
data "doit_cloud_diagram_snapshots" "test" {
  id     = local.first_layer_id
  limit  = %d
  offset = %d
  sort   = %q
}
`, limit, offset, sort)
}

func testAccCloudDiagramSnapshotsDataSourceConfigDirect(layerID string) string {
	return fmt.Sprintf(`
data "doit_cloud_diagram_snapshots" "test" {
  id = %q
}
`, layerID)
}
