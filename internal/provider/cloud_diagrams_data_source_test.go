package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams.test", "cloud_diagrams.#"),
				),
			},
		},
	})
}

func TestAccCloudDiagramsDataSource_WithMatch(t *testing.T) {
	resourceID := os.Getenv("TEST_CLOUD_DIAGRAM_RESOURCE")
	if resourceID == "" {
		t.Skip("TEST_CLOUD_DIAGRAM_RESOURCE environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramsDataSourceConfigWithMatch(resourceID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagrams.test", "id"),
					// This resource ID is known to have at least one diagram.
					resource.TestCheckResourceAttrWith("data.doit_cloud_diagrams.test", "cloud_diagrams.#", func(value string) error {
						if value == "0" {
							return fmt.Errorf("expected at least 1 diagram, got 0")
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccCloudDiagramsDataSourceConfig() string {
	return `
data "doit_cloud_diagrams" "test" {
  resources = ["//compute.googleapis.com/projects/nonexistent-project-12345"]
}
`
}

func testAccCloudDiagramsDataSourceConfigWithMatch(resourceID string) string {
	return fmt.Sprintf(`
data "doit_cloud_diagrams" "test" {
  resources = [%q]
}
`, resourceID)
}
