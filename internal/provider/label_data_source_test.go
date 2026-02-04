package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelDataSource_Basic(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelDataSourceConfig(n),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_label.test", "id",
						"doit_label.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_label.test", "name",
						"doit_label.test", "name"),
					resource.TestCheckResourceAttrPair(
						"data.doit_label.test", "color",
						"doit_label.test", "color"),
				),
			},
		},
	})
}

func testAccLabelDataSourceConfig(n int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "test-label-ds-%d"
  color = "blue"
}

data "doit_label" "test" {
  id = doit_label.test.id
}
`, n)
}

func TestAccLabelDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading label|not found|404`),
			},
		},
	})
}

func testAccLabelDataSourceNotFoundConfig() string {
	return `
data "doit_label" "notfound" {
  id = "nonexistent-label-id"
}
`
}
