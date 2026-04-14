package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSupportRequestCommentsDataSource_Basic(t *testing.T) {
	t.Skip("Skipped: support requests API returns 400 'Failed to get tickets'")
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestCommentsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_support_request_comments.test", "comments.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccSupportRequestCommentsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccSupportRequestCommentsDataSourceConfig() string {
	return `
data "doit_support_requests" "all" {
  max_results = 1
}

data "doit_support_request_comments" "test" {
  ticket_id = data.doit_support_requests.all.tickets[0].id
}
`
}
