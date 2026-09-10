package provider_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSupportRequestCommentsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestCommentsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_support_request_comments.test", "comments.#"),
					testCheckSupportRequestCommentsAttributes("data.doit_support_request_comments.test"),
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

func testCheckSupportRequestCommentsAttributes(dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", dataSourceName)
		}

		countStr, ok := rs.Primary.Attributes["comments.#"]
		if !ok {
			return fmt.Errorf("comments.# attribute not found")
		}

		count, err := strconv.Atoi(countStr)
		if err != nil {
			return fmt.Errorf("failed to parse comments.#: %w", err)
		}

		if count > 0 {
			val, ok := rs.Primary.Attributes["comments.0.public"]
			if !ok {
				return fmt.Errorf("comments.0.public is not set")
			}
			if val != "true" && val != "false" {
				return fmt.Errorf("comments.0.public expected 'true' or 'false', got %q", val)
			}
		}

		return nil
	}
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
