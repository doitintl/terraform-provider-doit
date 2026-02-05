package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAlertDataSource_Basic(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceConfig(n),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "id",
						"doit_alert.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "name",
						"doit_alert.test", "name"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.time_interval",
						"doit_alert.test", "config.time_interval"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.value",
						"doit_alert.test", "config.value"),
				),
			},
		},
	})
}

func testAccAlertDataSourceConfig(n int) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name = "test-alert-ds-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, n)
}

func TestAccAlertDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccAlertDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading alert|not found|404`),
			},
		},
	})
}

func testAccAlertDataSourceNotFoundConfig() string {
	return `
data "doit_alert" "notfound" {
  id = "nonexistent-alert-id"
}
`
}
