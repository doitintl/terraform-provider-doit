package provider_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAnnotationDataSource_Basic(t *testing.T) {
	n := rand.Int()                                                      //nolint:gosec // Weak random is fine for test data
	timestamp := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339) // Yesterday in UTC

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationDataSourceConfig(n, timestamp),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "id",
						"doit_annotation.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "content",
						"doit_annotation.test", "content"),
					resource.TestCheckResourceAttrPair(
						"data.doit_annotation.test", "timestamp",
						"doit_annotation.test", "timestamp"),
				),
			},
		},
	})
}

func testAccAnnotationDataSourceConfig(n int, timestamp string) string {
	return fmt.Sprintf(`
resource "doit_annotation" "test" {
  content   = "Test annotation content %d"
  timestamp = "%s"
}

data "doit_annotation" "test" {
  id = doit_annotation.test.id
}
`, n, timestamp)
}

func TestAccAnnotationDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAnnotationDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading annotation|not found|404`),
			},
		},
	})
}

func testAccAnnotationDataSourceNotFoundConfig() string {
	return `
data "doit_annotation" "notfound" {
  id = "nonexistent-annotation-id"
}
`
}
