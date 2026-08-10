package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

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
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.name"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.status"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.confidence"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.amount.amount"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.amount.currency"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.scope_chips.#"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.scope_chips.0.key"),
					resource.TestCheckResourceAttrSet("data.doit_budget_suggestions.test", "items.0.scope_chips.0.values.#"),
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
