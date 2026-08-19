package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCustomerDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton: shares customer state with resource tests
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCustomerDataSourceConfig("nonexistent-customer-id-12345"),
				ExpectError: regexp.MustCompile(`(Error Reading Customer|Could not read customer)`),
			},
		},
	})
}

func TestAccCustomerDataSource_Basic(t *testing.T) {
	customerID := os.Getenv("TEST_CUSTOMER_ID")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // singleton: shares customer state with resource tests
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomerDataSourceConfig(customerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_customer.test", "id", customerID),
					resource.TestCheckResourceAttr("data.doit_customer.test", "customer_id", customerID),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "name"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "primary_domain"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "domains.#"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "settings.currency"),
					resource.TestCheckResourceAttrSet("data.doit_customer.test", "settings.allowed_invite_domains.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccCustomerDataSourceConfig(customerID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCustomerDataSourceConfig(customerID string) string {
	return fmt.Sprintf(`
data "doit_customer" "test" {
  customer_id = %q
}
`, customerID)
}
