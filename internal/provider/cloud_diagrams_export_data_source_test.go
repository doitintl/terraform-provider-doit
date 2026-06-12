package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestAccCloudDiagramsExportDataSource_LinkOriginDestination(t *testing.T) {
	config := testAccCloudDiagramsExportLayerDiscovery() + `
data "doit_cloud_diagrams_export" "test" {
  id = local.first_layer_id
}

# If there are links, verify origin and destination are populated.
output "has_links" {
  value = length(data.doit_cloud_diagrams_export.test.links) > 0
}

output "first_link_origin_id" {
  value = length(data.doit_cloud_diagrams_export.test.links) > 0 ? data.doit_cloud_diagrams_export.test.links[0].origin._id : "no_links"
}

output "first_link_dest_id" {
  value = length(data.doit_cloud_diagrams_export.test.links) > 0 ? data.doit_cloud_diagrams_export.test.links[0].destination._id : "no_links"
}

output "first_link_origin_type" {
  value = length(data.doit_cloud_diagrams_export.test.links) > 0 ? data.doit_cloud_diagrams_export.test.links[0].origin.type : "no_links"
}

output "first_link_dest_type" {
  value = length(data.doit_cloud_diagrams_export.test.links) > 0 ? data.doit_cloud_diagrams_export.test.links[0].destination.type : "no_links"
}`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_export.test", "links.#"),
					testCheckLinkOriginDestination("data.doit_cloud_diagrams_export.test"),
				),
			},
		},
	})
}

func testCheckLinkOriginDestination(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}

		linkCount := rs.Primary.Attributes["links.#"]
		if linkCount == "" || linkCount == "0" {
			return nil
		}

		for _, field := range []string{
			"links.0.origin._id",
			"links.0.origin.type",
			"links.0.origin.scheme_id",
			"links.0.origin.ss_id",
			"links.0.destination._id",
			"links.0.destination.type",
			"links.0.destination.scheme_id",
			"links.0.destination.ss_id",
		} {
			if val, exists := rs.Primary.Attributes[field]; !exists || val == "" {
				return fmt.Errorf("expected %s to be set, got %q", field, val)
			}
		}
		return nil
	}
}
