package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccBudgetFixedLifecycleConfig is a fixed budget (30 days) whose growth,
// rollover and use_prev_spend settings come from settings.
func testAccBudgetFixedLifecycleConfig(name, settings string) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name         = %q
  currency     = "USD"
  type         = "fixed"
  start_period = local.start_period
  end_period   = local.start_period + (30 * 24 * 60 * 60 * 1000)
%s
  scopes = [
    {
      type   = "allocation_rule"
      id     = "allocation_rule"
      mode   = "is"
      values = [%q]
    }
  ]
  collaborators = [
    { email = %q, role = "owner" },
  ]
}
`, budgetStartPeriod(), name, settings, testAttribution(), testUser())
}

// TestAccBudget_GrowthFixedValidation checks at plan time what the API rejects
// for growth, rollover, use_prev_spend and fixed budgets. The API answers each
// of these with 400 on create and on update. Unknown values defer the check to
// apply, and what a fixed budget reads back (growth off, rate 0, disabled
// rollover, use_prev_spend false) stays valid.
func TestAccBudget_GrowthFixedValidation(t *testing.T) {
	cases := []struct {
		name       string
		fixed      bool
		settings   string
		dependency string
		wantError  string
	}{
		{"allow_growth false with rate", false, "amount = 100\n  allow_growth = false\n  growth_per_period = 5", "", `growth_per_period must be 0 when allow_growth is false`},
		{"negative rate", false, "amount = 100\n  growth_per_period = -1", "", `growth_per_period must not be negative`},
		{"allow_growth false, rate 0", false, "amount = 100\n  allow_growth = false\n  growth_per_period = 0", "", ""},
		{"rate without allow_growth", false, "amount = 100\n  growth_per_period = 5", "", ""},
		{"unknown rate", false, "amount = 100\n  allow_growth = false\n  growth_per_period = terraform_data.dep.output", `resource "terraform_data" "dep" { input = 5 }`, ""},

		{"use_prev_spend with rollover", false, "use_prev_spend = true\n  rollover = { enabled = true, limit = 2 }", "", `Rollover cannot be enabled together with use_prev_spend = true`},
		{"use_prev_spend with disabled rollover", false, "use_prev_spend = true\n  rollover = { enabled = false, limit = 0 }", "", ""},
		{"unknown use_prev_spend with rollover", false, "use_prev_spend = terraform_data.dep.output\n  rollover = { enabled = true, limit = 2 }", `resource "terraform_data" "dep" { input = true }`, ""},

		{"fixed with allow_growth", true, "amount = 100\n  allow_growth = true", "", `allow_growth cannot be true when type is "fixed"`},
		{"fixed with rate", true, "amount = 100\n  growth_per_period = 5", "", `growth_per_period must be 0 when type is "fixed"`},
		{"fixed with rollover", true, "amount = 100\n  rollover = { enabled = true, limit = 2 }", "", `Rollover cannot be enabled when type is "fixed"`},
		{"fixed with use_prev_spend", true, "use_prev_spend = true", "", `use_prev_spend cannot be true when type is "fixed"`},
		{"fixed with read-back values", true, "amount = 100\n  allow_growth = false\n  growth_per_period = 0\n  use_prev_spend = false\n  rollover = { enabled = false, limit = 0 }", "", ""},
		{"fixed, unknown allow_growth", true, "amount = 100\n  allow_growth = terraform_data.dep.output", `resource "terraform_data" "dep" { input = true }`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			typeSettings := `type          = "recurring"
  time_interval = "month"`
			if tc.fixed {
				typeSettings = `type       = "fixed"
  end_period = 1761955200000`
			}
			step := resource.TestStep{
				Config: fmt.Sprintf(`
%s
resource "doit_budget" "this" {
  name         = "test-budget-growth-fixed-validation"
  currency     = "USD"
  start_period = 1759276800000
  %s
  %s
  scopes = [{
    id     = "allocation_rule"
    type   = "allocation_rule"
    mode   = "is"
    values = [%q]
  }]
}
`, tc.dependency, typeSettings, tc.settings, testAttribution()),
				PlanOnly: true,
			}
			if tc.wantError == "" {
				step.ExpectNonEmptyPlan = true
			} else {
				step.ExpectError = budgetErrorPattern(tc.wantError)
			}
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}

// TestAccBudget_UpdateInfersAllowGrowth sets a growth rate on a budget with
// growth off. Create turns growth on from a rate, but the API does not do it on
// update: it keeps the stored allow_growth, and a rate above 0 next to a stored
// false is a 400. The omitted allow_growth plans as unknown, and the request
// sends true for the rate.
func TestAccBudget_UpdateInfersAllowGrowth(t *testing.T) {
	name := fmt.Sprintf("test-infer-growth-%d", acctest.RandInt())
	noGrowth := testAccBudgetLifecycleConfig(name, "  amount = 100")
	rate := testAccBudgetLifecycleConfig(name, "  amount = 100\n  growth_per_period = 3")
	off := testAccBudgetLifecycleConfig(name, "  amount = 100\n  allow_growth = false")

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: noGrowth,
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("allow_growth", knownvalue.Bool(false)),
					budgetState("growth_per_period", knownvalue.Float64Exact(0)),
				},
			},
			budgetDriftStep(noGrowth),
			// Omitting allow_growth while the rate changes plans it unknown.
			{
				Config: rate,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_budget.this", plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue("doit_budget.this", tfjsonpath.New("allow_growth")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("allow_growth", knownvalue.Bool(true)),
					budgetState("growth_per_period", knownvalue.Float64Exact(3)),
				},
			},
			budgetDriftStep(rate),
			// Turning growth off: the omitted rate is its default 0, sent with
			// allow_growth = false in the same request.
			{
				Config:           off,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("allow_growth", knownvalue.Bool(false)),
					budgetState("growth_per_period", knownvalue.Float64Exact(0)),
				},
			},
			budgetDriftStep(off),
		},
	})
}

// TestAccBudget_GrowthRolloverToFixed switches a recurring budget with growth
// and rollover to a fixed one. The API rejects a fixed budget with either, and
// a switch has to turn both off in the same request. The configuration of a
// fixed budget has neither, so the request builder sends the off values.
func TestAccBudget_GrowthRolloverToFixed(t *testing.T) {
	name := fmt.Sprintf("test-to-fixed-%d", acctest.RandInt())
	recurring := testAccBudgetLifecycleConfig(name, "  amount = 100\n  growth_per_period = 2\n  rollover = { enabled = true, limit = 3 }")
	fixed := testAccBudgetFixedLifecycleConfig(name, "  amount = 100")
	backToRecurring := testAccBudgetLifecycleConfig(name, "  amount = 100\n  growth_per_period = 2\n  rollover = { enabled = true, limit = 3 }")

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: recurring,
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("allow_growth", knownvalue.Bool(true)),
					budgetState("rollover", knownvalue.ObjectExact(map[string]knownvalue.Check{
						"enabled": knownvalue.Bool(true),
						"limit":   knownvalue.Int64Exact(3),
					})),
				},
			},
			budgetDriftStep(recurring),
			{
				Config:           fixed,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("type", knownvalue.StringExact("fixed")),
					budgetState("allow_growth", knownvalue.Bool(false)),
					budgetState("growth_per_period", knownvalue.Float64Exact(0)),
					budgetState("rollover", knownvalue.Null()),
					budgetState("use_prev_spend", knownvalue.Bool(false)),
				},
			},
			budgetDriftStep(fixed),
			// And back: a recurring budget may use both again.
			{
				Config:           backToRecurring,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("type", knownvalue.StringExact("recurring")),
					budgetState("allow_growth", knownvalue.Bool(true)),
					budgetState("growth_per_period", knownvalue.Float64Exact(2)),
				},
			},
			budgetDriftStep(backToRecurring),
		},
	})
}

// TestAccBudget_UsePrevSpendOff turns use_prev_spend off. The amount of a
// previous-spend budget is last period's spend, so the API needs the amount to
// keep in the same request, unless seasonal amounts replace it. The check runs
// at plan time against the stored budget.
func TestAccBudget_UsePrevSpendOff(t *testing.T) {
	name := fmt.Sprintf("test-prev-spend-off-%d", acctest.RandInt())
	prevSpend := testAccBudgetLifecycleConfig(name, "  use_prev_spend = true")
	offWithoutAmount := testAccBudgetLifecycleConfig(name, "  use_prev_spend = false")
	omittedWithoutAmount := testAccBudgetLifecycleConfig(name, "")
	offWithAmount := testAccBudgetLifecycleConfig(name, "  use_prev_spend = false\n  amount = 200")
	offWithSeasonal := testAccBudgetLifecycleConfig(name, "  use_prev_spend = false\n  seasonal_amounts = "+budgetMonthlySeasonal)

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: prevSpend,
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("use_prev_spend", knownvalue.Bool(true)),
				},
			},
			budgetDriftStep(prevSpend),
			{
				Config:      offWithoutAmount,
				PlanOnly:    true,
				ExpectError: budgetErrorPattern("Turning use_prev_spend off needs the amount to keep"),
			},
			{
				Config:      omittedWithoutAmount,
				PlanOnly:    true,
				ExpectError: budgetErrorPattern("Turning use_prev_spend off needs the amount to keep"),
			},
			{
				Config:           offWithAmount,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("use_prev_spend", knownvalue.Bool(false)),
					budgetState("amount", knownvalue.Float64Exact(200)),
				},
			},
			budgetDriftStep(offWithAmount),
			// On again, then off with seasonal amounts as the replacement.
			{
				Config:           prevSpend,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("use_prev_spend", knownvalue.Bool(true)),
				},
			},
			budgetDriftStep(prevSpend),
			{
				Config:           offWithSeasonal,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("use_prev_spend", knownvalue.Bool(false)),
					budgetState("seasonal_amounts", knownvalue.ListSizeExact(12)),
				},
			},
			budgetDriftStep(offWithSeasonal),
		},
	})
}

// TestAccBudget_UsePrevSpendImport imports a previous-spend budget. The
// configuration generated for it holds the amount the API reads back (last
// period's spend) next to use_prev_spend = true, and must validate and plan as
// a no-op import.
func TestAccBudget_UsePrevSpendImport(t *testing.T) {
	name := fmt.Sprintf("test-prev-spend-import-%d", acctest.RandInt())
	config := testAccBudgetLifecycleConfig(name, "  use_prev_spend = true")

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:    "doit_budget.this",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
		},
	})
}
