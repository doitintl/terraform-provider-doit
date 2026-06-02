package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramSearchDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramSearchDataSourceConfig("nonexistent-resource-12345"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_search.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_search.test", "scheme.#"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_search.test", "component.#"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_search.test", "prop.#"),
				),
			},
		},
	})
}

func TestAccCloudDiagramSearchDataSource_WithMatch(t *testing.T) {
	query := os.Getenv("TEST_CLOUD_DIAGRAM_RESOURCE")
	if query == "" {
		t.Skip("TEST_CLOUD_DIAGRAM_RESOURCE environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramSearchDataSourceConfig(query),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_search.test", "id"),
					// The query is a known resource ID that should match at least one result
					// across any of the three categories.
				),
			},
		},
	})
}

func TestAccCloudDiagramSearchDataSource_WithSize(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramSearchDataSourceConfigWithSize("test", 5),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_search.test", "id"),
				),
			},
		},
	})
}

func testAccCloudDiagramSearchDataSourceConfig(query string) string {
	return fmt.Sprintf(`
data "doit_cloud_diagram_search" "test" {
  query = %q
}
`, query)
}

func testAccCloudDiagramSearchDataSourceConfigWithSize(query string, size int) string {
	return fmt.Sprintf(`
data "doit_cloud_diagram_search" "test" {
  query = %q
  size  = %d
}
`, query, size)
}
