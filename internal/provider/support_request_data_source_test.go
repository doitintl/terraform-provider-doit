package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSupportRequestDataSource_Basic(t *testing.T) {
	t.Skip("Skipped: support requests API returns 400 'Failed to get tickets'")
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_support_request.test", "id"),
					resource.TestCheckResourceAttrSet("data.doit_support_request.test", "subject"),
					resource.TestCheckResourceAttrSet("data.doit_support_request.test", "status"),
					resource.TestCheckResourceAttrSet("data.doit_support_request.test", "requester"),
					resource.TestCheckResourceAttrSet("data.doit_support_request.test", "create_time"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccSupportRequestDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccSupportRequestDataSourceConfig() string {
	return `
data "doit_support_requests" "all" {
  max_results = 1
}

data "doit_support_request" "test" {
  ticket_id = data.doit_support_requests.all.tickets[0].id
}
`
}
