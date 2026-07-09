package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSupportRequestDataSource_Basic(t *testing.T) {
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
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_support_request.test",
						tfjsonpath.New("tags"),
						knownvalue.NotNull(),
					),
				},
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

func TestAccSupportRequestDataSource_WithTags(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // shares ticket with tags resource tests
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestTagsOnlyConfig(),
			},
			{
				Config: testAccSupportRequestDataSourceWithTagsConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_support_request.tagged",
						tfjsonpath.New("tags"),
						knownvalue.ListSizeExact(1),
					),
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

func testAccSupportRequestTagsOnlyConfig() string {
	return `
data "doit_support_requests" "all" {
  max_results = 1
}

resource "doit_support_request_tags" "fixture" {
  ticket_id = data.doit_support_requests.all.tickets[0].id
  tags      = ["tf-test-ds-verify"]
}
`
}

func testAccSupportRequestDataSourceWithTagsConfig() string {
	return `
data "doit_support_requests" "all" {
  max_results = 1
}

resource "doit_support_request_tags" "fixture" {
  ticket_id = data.doit_support_requests.all.tickets[0].id
  tags      = ["tf-test-ds-verify"]
}

data "doit_support_request" "tagged" {
  ticket_id = doit_support_request_tags.fixture.ticket_id
}
`
}
