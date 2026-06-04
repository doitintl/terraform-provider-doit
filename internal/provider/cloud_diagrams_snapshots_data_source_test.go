package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsSnapshotsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsSnapshotsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshots.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshots.test", "cloud_diagrams_snapshots.#"),
				),
			},
			{
				Config:   testAccCloudDiagramsSnapshotsDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsSnapshotsDataSource_WithOptions(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsSnapshotsDataSourceConfigWithOptions(2, 0, "-createdAt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshots.test", "id"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_snapshots.test", "limit", "2"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_snapshots.test", "offset", "0"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_snapshots.test", "sort", "-createdAt"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_snapshots.test", "cloud_diagrams_snapshots.#"),
				),
			},
			{
				Config:   testAccCloudDiagramsSnapshotsDataSourceConfigWithOptions(2, 0, "-createdAt"),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsSnapshotsDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudDiagramsSnapshotsDataSourceConfigDirect("nonexistent-layer-id-12345"),
				ExpectError: regexp.MustCompile(`(?i)not found|404|Cloud Diagram layer`),
			},
		},
	})
}

// testAccCloudDiagramsSnapshotsLayerDiscovery returns the HCL block that
// discovers the first available layer ID from doit_cloud_diagrams_schemes.
func testAccCloudDiagramsSnapshotsLayerDiscovery() string {
	return `
# Discover the first available layer ID from the schemes endpoint.
data "doit_cloud_diagrams_schemes" "discovery" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.discovery.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.discovery.scheme[local.first_scheme_key].statussheet[0].ssid
}
`
}

func testAccCloudDiagramsSnapshotsDataSourceConfig() string {
	return testAccCloudDiagramsSnapshotsLayerDiscovery() + `
data "doit_cloud_diagrams_snapshots" "test" {
  id = local.first_layer_id
}
`
}

func testAccCloudDiagramsSnapshotsDataSourceConfigWithOptions(limit, offset int, sort string) string {
	return testAccCloudDiagramsSnapshotsLayerDiscovery() + fmt.Sprintf(`
data "doit_cloud_diagrams_snapshots" "test" {
  id     = local.first_layer_id
  limit  = %d
  offset = %d
  sort   = %q
}
`, limit, offset, sort)
}

func testAccCloudDiagramsSnapshotsDataSourceConfigDirect(layerID string) string {
	return fmt.Sprintf(`
data "doit_cloud_diagrams_snapshots" "test" {
  id = %q
}
`, layerID)
}
