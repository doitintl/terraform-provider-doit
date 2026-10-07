package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccServiceAccountDataSource_Basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-sa-ds")
	config := testAccServiceAccountDataSourceConfig(name)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.doit_service_account.test", "id", "doit_service_account.test", "id"),
					resource.TestCheckResourceAttrPair("data.doit_service_account.test", "name", "doit_service_account.test", "name"),
					resource.TestCheckResourceAttrPair("data.doit_service_account.test", "description", "doit_service_account.test", "description"),
					resource.TestCheckResourceAttrPair("data.doit_service_account.test", "permissions.#", "doit_service_account.test", "permissions.#"),
					resource.TestCheckResourceAttrSet("data.doit_service_account.test", "customer_id"),
					resource.TestCheckResourceAttrSet("data.doit_service_account.test", "create_time"),
					resource.TestCheckResourceAttrSet("data.doit_service_account.test", "etag"),
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

func testAccServiceAccountDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_service_account" "test" {
  name        = %q
  description = "Data source acceptance test"
  permissions = ["budgetsReadOnly"]
}

data "doit_service_account" "test" {
  id = doit_service_account.test.id
}
`, name)
}

func TestAccServiceAccountDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      `data "doit_service_account" "missing" { id = "nonexistent-service-account-id" }`,
			ExpectError: regexp.MustCompile(`(?i)error reading service account|404`),
		}},
	})
}
