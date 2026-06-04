package provider_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCloudDiagramsSchemesDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Empty body returns all accessible diagrams (scheme map only).
				Config: testAccCloudDiagramsSchemesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_schemes.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_schemes.test", "scheme.%"),
				),
			},
			{
				// PlanOnly idempotency check.
				Config:   testAccCloudDiagramsSchemesDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsSchemesDataSource_WithSchemeID(t *testing.T) {
	// This test discovers a scheme ID from the overview call, then fetches
	// that specific scheme with components enabled.
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Step 1: Discover a scheme ID from the overview response,
				// then use it to fetch components for that specific diagram.
				Config: testAccCloudDiagramsSchemesDataSourceConfigChained(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_schemes.overview", "scheme.%"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_schemes.detailed", "id"),
				),
			},
			{
				Config:   testAccCloudDiagramsSchemesDataSourceConfigChained(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsSchemesDataSource_WithSkipEmpty(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsSchemesDataSourceConfigWithSkipEmpty(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams_schemes.test", "id"),
				),
			},
			{
				Config:   testAccCloudDiagramsSchemesDataSourceConfigWithSkipEmpty(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramsSchemesDataSource_HasDiagramWithLayers(t *testing.T) {
	// Validates that at least one diagram has layers (statussheet entries),
	// confirming the nested scheme → statussheet mapping works.
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsSchemesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckAtLeastOneDiagramWithLayers("data.doit_cloud_diagrams_schemes.test"),
				),
			},
		},
	})
}

func testAccCloudDiagramsSchemesDataSourceConfig() string {
	return `
data "doit_cloud_diagrams_schemes" "test" {
}
`
}

func testAccCloudDiagramsSchemesDataSourceConfigChained() string {
	// Use the overview to discover a scheme ID, then fetch it with components.
	// The locals block extracts the first key from the scheme map.
	return `
data "doit_cloud_diagrams_schemes" "overview" {
}

locals {
  scheme_ids = keys(data.doit_cloud_diagrams_schemes.overview.scheme)
  first_id   = length(local.scheme_ids) > 0 ? [local.scheme_ids[0]] : []
}

data "doit_cloud_diagrams_schemes" "detailed" {
  scheme_ids = local.first_id
  components = true
}
`
}

func testAccCloudDiagramsSchemesDataSourceConfigWithSkipEmpty() string {
	return `
data "doit_cloud_diagrams_schemes" "test" {
  skip_empty = true
}
`
}

// testCheckAtLeastOneDiagramWithLayers walks the scheme map attributes and
// verifies at least one diagram has a non-empty statussheet list.
func testCheckAtLeastOneDiagramWithLayers(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}

		// Scan all attributes for scheme.*.statussheet.# > 0.
		const suffix = ".statussheet.#"
		for key, val := range rs.Primary.Attributes {
			if strings.HasSuffix(key, suffix) {
				if val != "0" {
					return nil // Found a diagram with at least one layer.
				}
			}
		}

		return fmt.Errorf("expected at least one diagram with layers, but none found")
	}
}
