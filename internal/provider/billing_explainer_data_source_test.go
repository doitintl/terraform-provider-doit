package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccBillingExplainerDataSource_Basic(t *testing.T) {
	invoiceMonth := os.Getenv("TEST_BILLING_EXPLAINER_INVOICE_MONTH")
	if invoiceMonth == "" {
		t.Skip("TEST_BILLING_EXPLAINER_INVOICE_MONTH environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBillingExplainerDataSourceConfig(invoiceMonth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_billing_explainer.test", "invoice_month", invoiceMonth),
					resource.TestCheckResourceAttrSet("data.doit_billing_explainer.test", "customer_id"),
					resource.TestCheckResourceAttrSet("data.doit_billing_explainer.test", "update_time"),
					resource.TestCheckResourceAttrSet("data.doit_billing_explainer.test", "doit_credits.amount"),
					resource.TestCheckResourceAttrSet("data.doit_billing_explainer.test", "payers.%"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccBillingExplainerDataSourceConfig(invoiceMonth),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBillingExplainerDataSourceConfig(invoiceMonth string) string {
	return fmt.Sprintf(`
data "doit_billing_explainer" "test" {
  invoice_month = %[1]q
}
`, invoiceMonth)
}
