package provider_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccBudgetSuggestionsDataSource_Basic intentionally avoids asserting on
// items.0.* — the pending-suggestions set is real account state that can
// legitimately empty out at any time (suggestions get accepted/dismissed, or
// simply age out), so indexing into the first item would make this test flaky
// whenever that happens even though the API is behaving correctly. Nested
// field mapping (amount, scope_chips, config, null handling) is covered
// deterministically in TestMapBudgetSuggestionsItems_* instead.
func TestAccBudgetSuggestionsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetSuggestionsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "row_count"),
					testAccCheckBudgetSuggestionsRowCountMatchesItems("data.doit_budget_suggestions.test"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccBudgetSuggestionsDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccBudgetSuggestionsDataSourceConfig() string {
	return `
data "doit_budget_suggestions" "test" {}
`
}

// testAccCheckBudgetSuggestionsRowCountMatchesItems verifies row_count is
// consistent with the returned items list, whatever its size (including 0).
func testAccCheckBudgetSuggestionsRowCountMatchesItems(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		rowCount, err := strconv.Atoi(rs.Primary.Attributes["row_count"])
		if err != nil {
			return fmt.Errorf("row_count is not a number: %w", err)
		}

		itemsCount, err := strconv.Atoi(rs.Primary.Attributes["items.#"])
		if err != nil {
			return fmt.Errorf("items.# is not a number: %w", err)
		}

		if rowCount != itemsCount {
			return fmt.Errorf("row_count (%d) does not match items.# (%d)", rowCount, itemsCount)
		}

		return nil
	}
}
