package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-label-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelDataSourceConfig(rName),
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

func testAccLabelDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = %q
  color = "blue"
}

data "doit_label" "test" {
  id = doit_label.test.id
}
`, name)
}

func TestAccLabelDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
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
