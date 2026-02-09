package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAlertDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceConfig(rName),
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

func testAccAlertDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name = %q
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
`, name)
}

func TestAccAlertDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
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

// TestAccAlertDataSource_WithScopes tests reading an alert with scopes configured
// to exercise the mapScopeToModel function path.
func TestAccAlertDataSource_WithScopes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert-ds-scopes")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertDataSourceWithScopesConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "id",
						"doit_alert.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_alert.test", "config.scopes.#",
						"doit_alert.test", "config.scopes.#"),
					resource.TestCheckResourceAttr(
						"data.doit_alert.test", "config.scopes.0.type", "fixed"),
					resource.TestCheckResourceAttr(
						"data.doit_alert.test", "config.scopes.0.id", "cloud_provider"),
				),
			},
		},
	})
}

func testAccAlertDataSourceWithScopesConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_alert" "test" {
  name = %q
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
    scopes = [
      {
        type   = "fixed"
        id     = "cloud_provider"
        mode   = "is"
        values = ["amazon-web-services"]
      }
    ]
  }
}

data "doit_alert" "test" {
  id = doit_alert.test.id
}
`, name)
}
