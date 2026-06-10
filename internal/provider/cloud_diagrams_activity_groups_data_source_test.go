package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsActivityGroupsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsActivityGroupsLayerDiscovery() + `
data "doit_cloud_diagrams_activity_groups" "test" {
  ss_id = local.first_layer_id
  limit = 3
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_activity_groups.test", "ss_id", "69de517081ee05bd185569d2"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_activity_groups.test", "cloud_diagrams_activity_groups.#"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsActivityGroupsDataSource_WithOffset(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsActivityGroupsLayerDiscovery() + `
data "doit_cloud_diagrams_activity_groups" "test" {
  ss_id  = local.first_layer_id
  limit  = 2
  offset = 1
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_activity_groups.test", "limit", "2"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_activity_groups.test", "offset", "1"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsActivityGroupsDataSource_NoResults(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsActivityGroupsLayerDiscovery() + `
data "doit_cloud_diagrams_activity_groups" "test" {
  ss_id  = local.first_layer_id
  offset = 999999
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagrams_activity_groups.test", "cloud_diagrams_activity_groups.#", "0"),
				),
			},
		},
	})
}

func testAccCloudDiagramsActivityGroupsLayerDiscovery() string {
	return `
# Discover the first available layer ID from the schemes endpoint.
data "doit_cloud_diagrams_schemes" "discovery" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.discovery.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.discovery.scheme[local.first_scheme_key].statussheet[0].ssid
}
`
}
