package provider_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
					testCheckAtLeastOneSearchResult("data.doit_cloud_diagram_search.test"),
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

// testCheckAtLeastOneSearchResult verifies that the search returned at least
// one result in any of the three categories (scheme, component, prop).
func testCheckAtLeastOneSearchResult(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}

		for _, attr := range []string{"scheme.#", "component.#", "prop.#"} {
			val, ok := rs.Primary.Attributes[attr]
			if !ok {
				continue
			}
			count, err := strconv.Atoi(val)
			if err != nil {
				continue
			}
			if count > 0 {
				return nil
			}
		}

		return fmt.Errorf("expected at least one result in scheme, component, or prop, but all were empty")
	}
}
