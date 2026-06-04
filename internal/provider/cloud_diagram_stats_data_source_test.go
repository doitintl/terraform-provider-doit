package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramStatsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramStatsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_stats.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_stats.test", "cloud_diagram_stats.#"),
				),
			},
			{
				Config:   testAccCloudDiagramStatsDataSourceConfig(),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCloudDiagramStatsDataSource_ShortPeriod(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramStatsDataSourceConfigShortPeriod(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_stats.test", "id"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_stats.test", "start", "2025-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_stats.test", "end", "2025-01-02T00:00:00Z"),
				),
			},
		},
	})
}

func testAccCloudDiagramStatsDataSourceConfig() string {
	return `
# Use a 30-day window to capture activity stats.
data "doit_cloud_diagram_stats" "test" {
  start = timeadd(plantimestamp(), "-720h")
  end   = plantimestamp()
}
`
}

func testAccCloudDiagramStatsDataSourceConfigShortPeriod() string {
	return `
# Use a narrow window (1 day in the past) to test with fewer results.
data "doit_cloud_diagram_stats" "test" {
  start = "2025-01-01T00:00:00Z"
  end   = "2025-01-02T00:00:00Z"
}
`
}
