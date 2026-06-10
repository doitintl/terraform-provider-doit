package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsExportDataSource_Basic(t *testing.T) {
	config := testAccCloudDiagramsExportLayerDiscovery() + `
data "doit_cloud_diagrams_export" "test" {
  id = local.first_layer_id
}`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.user"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.date"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.version"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "nodes.#"),
				),
			},
			// Re-read to confirm idempotent behavior.
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.user"),
				),
			},
		},
	})
}

func testAccCloudDiagramsExportLayerDiscovery() string {
	return `
# Discover the first available layer ID from the schemes endpoint.
data "doit_cloud_diagrams_schemes" "discovery" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.discovery.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.discovery.scheme[local.first_scheme_key].statussheet[0].ssid
}
`
}
