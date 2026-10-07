package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const testAccServiceAccountTokenDataSourceAddr = "data.doit_service_account_token.test"

// Each test creates its own service account because a service account holds at
// most 10 tokens and tests run in parallel.

func TestAccServiceAccountTokenDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-ds")
	config := testAccServiceAccountTokenDataSourceConfig(rName, `expires_time = "2099-01-01T00:00:00Z"`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccServiceAccountTokenDataSourceAddr, "id", testAccServiceAccountTokenAddr, "id"),
					resource.TestCheckResourceAttrPair(testAccServiceAccountTokenDataSourceAddr, "service_account_id", testAccServiceAccountTokenAddr, "service_account_id"),
					resource.TestCheckResourceAttrPair(testAccServiceAccountTokenDataSourceAddr, "customer_id", testAccServiceAccountTokenAddr, "customer_id"),
					resource.TestCheckResourceAttrPair(testAccServiceAccountTokenDataSourceAddr, "name", testAccServiceAccountTokenAddr, "name"),
					resource.TestCheckResourceAttrPair(testAccServiceAccountTokenDataSourceAddr, "state", testAccServiceAccountTokenAddr, "state"),
					resource.TestCheckResourceAttrPair(testAccServiceAccountTokenDataSourceAddr, "create_time", testAccServiceAccountTokenAddr, "create_time"),
					resource.TestCheckResourceAttr(testAccServiceAccountTokenDataSourceAddr, "expires_time", "2099-01-01T00:00:00Z"),
					// The credential is returned only by create and is not part of the data source.
					resource.TestCheckNoResourceAttr(testAccServiceAccountTokenDataSourceAddr, "access_token"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccServiceAccountTokenDataSource_Disabled(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-ds-dis")
	config := testAccServiceAccountTokenDataSourceConfig(rName, `state = "disabled"`)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServiceAccountTokenDataSourceAddr, "state", "disabled"),
					resource.TestCheckResourceAttrSet(testAccServiceAccountTokenDataSourceAddr, "expires_time"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccServiceAccountTokenDataSourceConfig(name, tokenExtra string) string {
	return testAccServiceAccountTokenConfig(name, "", tokenExtra) + `
data "doit_service_account_token" "test" {
  service_account_id = doit_service_account_token.this.service_account_id
  id                 = doit_service_account_token.this.id
}
`
}

func TestAccServiceAccountTokenDataSource_NotFound(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sat-ds-nf")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
resource "doit_service_account" "this" {
  name = %q
}

data "doit_service_account_token" "missing" {
  service_account_id = doit_service_account.this.id
  id                 = "nonexistent-token-id"
}
`, rName),
			ExpectError: regexp.MustCompile(`(?i)error reading service account token|404`),
		}},
	})
}
