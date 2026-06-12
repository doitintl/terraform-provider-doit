package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsStatussheetDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Chain: schemes → layer ID → discover component IDs → statussheet
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id        = local.first_layer_id
  node_ids  = local.node_ids
  group_ids = local.group_ids
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id        = local.first_layer_id
  node_ids  = local.node_ids
  group_ids = local.group_ids
}`,
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsStatussheetDataSource_MultipleComponentTypes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id        = local.first_layer_id
  node_ids  = local.node_ids
  group_ids = local.group_ids
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsStatussheetDataSource_WithProjection(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id        = local.first_layer_id
  node_ids  = local.node_ids
  group_ids = local.group_ids
  p         = "name icon color"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
				),
			},
		},
	})
}

// testAccCloudDiagramsStatussheetDiscovery chains from schemes to discover
// component IDs for the first available layer. Uses null instead of [] for
// empty lists to avoid triggering listvalidator.SizeAtLeast(1).
func testAccCloudDiagramsStatussheetDiscovery() string {
	return `
# Step 1: Get an overview of all diagrams.
data "doit_cloud_diagrams_schemes" "overview" {}

# Step 2: Pick the first diagram and its first layer.
locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.overview.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.overview.scheme[local.first_scheme_key].statussheet[0].ssid
}

# Step 3: Load the diagram with component data for that layer.
data "doit_cloud_diagrams_schemes" "with_components" {
  layer_ids = [local.first_layer_id]
}

# Step 4: Extract component IDs from the statussheet data.
# Use null (not []) when no IDs are found, so the SizeAtLeast(1) validator
# is not triggered on empty lists.
locals {
  ss_data   = data.doit_cloud_diagrams_schemes.with_components.statussheet[local.first_layer_id]
  node_ids  = local.ss_data != null && length(keys(local.ss_data.node)) > 0 ? keys(local.ss_data.node) : null
  group_ids = local.ss_data != null && length(keys(local.ss_data.group)) > 0 ? keys(local.ss_data.group) : null
}
`
}

func TestAccCloudDiagramsStatussheetDataSource_ComponentMapsIterable(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsStatussheetDiscovery() + `
data "doit_cloud_diagrams_statussheet" "test" {
  id       = local.first_layer_id
  node_ids = local.node_ids
}

# Iterate over all component maps to verify they are not null.
# keys() and length() would fail on null maps.
output "ss_node_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.node))
}

output "ss_element_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.element))
}

output "ss_group_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.group))
}

output "ss_link_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.link))
}

output "ss_attachment_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.attachment))
}

output "ss_combiner_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.combiner))
}

output "ss_note_key_count" {
  value = length(keys(data.doit_cloud_diagrams_statussheet.test.note))
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_statussheet.test", "id"),
					resource.TestMatchOutput("ss_node_key_count", regexp.MustCompile(`^\d+$`)),
					resource.TestMatchOutput("ss_element_key_count", regexp.MustCompile(`^\d+$`)),
					resource.TestMatchOutput("ss_group_key_count", regexp.MustCompile(`^\d+$`)),
					resource.TestMatchOutput("ss_link_key_count", regexp.MustCompile(`^\d+$`)),
					resource.TestMatchOutput("ss_attachment_key_count", regexp.MustCompile(`^\d+$`)),
					resource.TestMatchOutput("ss_combiner_key_count", regexp.MustCompile(`^\d+$`)),
					resource.TestMatchOutput("ss_note_key_count", regexp.MustCompile(`^\d+$`)),
				),
			},
		},
	})
}
