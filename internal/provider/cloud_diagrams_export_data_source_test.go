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
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.user"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.date"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.version"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "nodes.#"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "statussheet"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsExportDataSource_ConnectionsIterable(t *testing.T) {
	config := testAccCloudDiagramsExportLayerDiscovery() + `
data "doit_cloud_diagrams_export" "test" {
  id = local.first_layer_id
}

# Iterate over metadata.connections — must not error even when empty.
output "connection_keys" {
  value = keys(data.doit_cloud_diagrams_export.test.metadata.connections)
}

output "connection_count" {
  value = length(data.doit_cloud_diagrams_export.test.metadata.connections)
}`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "metadata.connections.%"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsExportDataSource_StatussheetNotNull(t *testing.T) {
	config := testAccCloudDiagramsExportLayerDiscovery() + `
data "doit_cloud_diagrams_export" "test" {
  id = local.first_layer_id
}

# Verify statussheet is valid JSON by decoding it.
output "statussheet_valid" {
  value = can(jsondecode(data.doit_cloud_diagrams_export.test.statussheet))
}`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "statussheet"),
					resource.TestCheckOutput("statussheet_valid", "true"),
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
