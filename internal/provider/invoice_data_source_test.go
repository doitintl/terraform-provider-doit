package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccInvoiceDataSource_Basic(t *testing.T) {
	invoiceID := os.Getenv("TEST_INVOICE_ID")
	if invoiceID == "" {
		t.Skip("TEST_INVOICE_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoiceDataSourceConfig(invoiceID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_invoice.test", "id", invoiceID),
					resource.TestCheckResourceAttrSet("data.doit_invoice.test", "currency"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccInvoiceDataSourceConfig(invoiceID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInvoiceDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_invoice" "test" {
  id = %[1]q
}
`, id)
}
