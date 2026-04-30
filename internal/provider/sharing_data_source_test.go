package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSharingDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sharing-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSharingDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify data source returns the same resource_id as the report
					resource.TestCheckResourceAttrPair(
						"data.doit_sharing.test", "id",
						"doit_report.test", "id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_sharing.test", "name",
						"doit_report.test", "name"),
					// Verify permissions are populated
					resource.TestCheckResourceAttr(
						"data.doit_sharing.test", "permissions.#", "1"),
					resource.TestCheckResourceAttr(
						"data.doit_sharing.test", "permissions.0.role", "owner"),
					// Verify resource_type is set correctly
					resource.TestCheckResourceAttr(
						"data.doit_sharing.test", "resource_type", "reports"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccSharingDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccSharingDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_report" "test" {
  name = %q

  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_range = {
      mode            = "last"
      amount          = 1
      unit            = "month"
      include_current = true
    }
  }
}

data "doit_sharing" "test" {
  resource_type = "reports"
  resource_id   = doit_report.test.id
}
`, name)
}

func TestAccSharingDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccSharingDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading resource sharing|not found|404`),
			},
		},
	})
}

func testAccSharingDataSourceNotFoundConfig() string {
	return `
data "doit_sharing" "notfound" {
  resource_type = "reports"
  resource_id   = "nonexistent-resource-id"
}
`
}
