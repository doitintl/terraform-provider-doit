package provider_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// budgetMonthlySeasonal holds twelve distinct amounts, so a wrong period or a
// fallback to another value would show.
const budgetMonthlySeasonal = "[10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120]"

// testAccBudgetLifecycleConfig is a monthly recurring budget whose amount
// settings (amount, seasonal_amounts, growth, rollover, use_prev_spend) come
// from settings.
func testAccBudgetLifecycleConfig(name, settings string) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = %q
  currency      = "USD"
  type          = "recurring"
  time_interval = "month"
  start_period  = local.start_period
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

func budgetTimeProvider() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"time": {Source: "hashicorp/time", VersionConstraint: "~> 0.13"},
	}
}

// budgetErrorPattern matches message in Terraform's output, which wraps long
// diagnostics at any space.
func budgetErrorPattern(message string) *regexp.Regexp {
	return regexp.MustCompile(strings.ReplaceAll(message, " ", `\s+`))
}

func budgetDriftStep(config string) resource.TestStep {
	return resource.TestStep{
		Config: config,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

func budgetUpdateChecks() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("doit_budget.this", plancheck.ResourceActionUpdate),
		},
	}
}

func budgetState(attribute string, check knownvalue.Check) statecheck.StateCheck {
	return statecheck.ExpectKnownValue("doit_budget.this", tfjsonpath.New(attribute), check)
}

// TestAccBudget_SeasonalLifecycle switches one budget between seasonal
// amounts, a fixed amount and use_prev_spend. A seasonal budget has no amount
// (the API rejects one and omits it on read), and omitting seasonal_amounts
// removes them, which the API only accepts together with amount or
// use_prev_spend.
func TestAccBudget_SeasonalLifecycle(t *testing.T) {
	n := acctest.RandInt()
	name := fmt.Sprintf("test-seasonal-%d", n)
	seasonal := testAccBudgetLifecycleConfig(name, "  seasonal_amounts = "+budgetMonthlySeasonal)
	seasonalWithDataSource := seasonal + `
data "doit_budget" "this" {
  id = doit_budget.this.id
}
`
	fixedRenamed := testAccBudgetLifecycleConfig(name+"-renamed", "  amount = 100")
	seasonalRenamed := testAccBudgetLifecycleConfig(name+"-renamed", "  seasonal_amounts = "+budgetMonthlySeasonal)
	prevSpend := testAccBudgetLifecycleConfig(name+"-renamed", "  use_prev_spend = true")

	seasonalChecks := []statecheck.StateCheck{
		budgetState("amount", knownvalue.Null()),
		budgetState("current_period_amount", knownvalue.NotNull()),
		budgetState("seasonal_amounts", knownvalue.ListSizeExact(12)),
		budgetState("allow_growth", knownvalue.Bool(false)),
		budgetState("rollover", knownvalue.Null()),
	}

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: seasonalWithDataSource,
				ConfigStateChecks: append(seasonalChecks,
					statecheck.ExpectKnownValue("data.doit_budget.this", tfjsonpath.New("amount"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_budget.this", tfjsonpath.New("current_period_amount"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_budget.this", tfjsonpath.New("seasonal_amounts"), knownvalue.ListSizeExact(12)),
				),
			},
			budgetDriftStep(seasonalWithDataSource),
			// Seasonal to a fixed amount, together with a rename: omitting
			// seasonal_amounts sends [] next to the amount.
			{
				Config:           fixedRenamed,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("amount", knownvalue.Float64Exact(100)),
					budgetState("current_period_amount", knownvalue.Float64Exact(100)),
					budgetState("seasonal_amounts", knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			budgetDriftStep(fixedRenamed),
			// Back to seasonal: the omitted amount is left out of the request.
			{
				Config:            seasonalRenamed,
				ConfigPlanChecks:  budgetUpdateChecks(),
				ConfigStateChecks: seasonalChecks,
			},
			budgetDriftStep(seasonalRenamed),
			// Seasonal to use_prev_spend, which the API also accepts for
			// removing seasonal amounts.
			{
				Config:           prevSpend,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("use_prev_spend", knownvalue.Bool(true)),
					budgetState("seasonal_amounts", knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			budgetDriftStep(prevSpend),
		},
	})
}

// TestAccBudget_SeasonalImport imports a seasonal budget. The configuration
// generated for it has no amount, allow_growth = false and growth_per_period
// = 0, and must pass the seasonal validators.
func TestAccBudget_SeasonalImport(t *testing.T) {
	name := fmt.Sprintf("test-seasonal-import-%d", acctest.RandInt())
	// alerts is set because a budget without alerts reads back alerts = [],
	// which budgetAlertsLengthValidator rejects in generated configuration
	// (#482).
	config := testAccBudgetLifecycleConfig(name, "  seasonal_amounts = "+budgetMonthlySeasonal+"\n  alerts = [{ percentage = 80 }]")

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:      "doit_budget.this",
				ImportState:       true,
				ImportStateVerify: true,
				// Import has no prior state to keep the aliased scope type
				// (see TestAccBudget_Import).
				ImportStateVerifyIgnore: []string{"scopes.0.id", "scopes.0.type"},
			},
			{
				ResourceName:    "doit_budget.this",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
		},
	})
}

// TestAccBudget_GrowthToSeasonal switches a budget with growth to seasonal
// amounts. An update keeps the stored allow_growth whatever growth_per_period
// says, so without allow_growth = false the switch fails at plan time, and
// with it the rate is kept but not applied.
func TestAccBudget_GrowthToSeasonal(t *testing.T) {
	name := fmt.Sprintf("test-growth-seasonal-%d", acctest.RandInt())
	growth := testAccBudgetLifecycleConfig(name, "  amount = 100\n  growth_per_period = 1")
	seasonalKeepsGrowth := testAccBudgetLifecycleConfig(name, "  growth_per_period = 1\n  seasonal_amounts = "+budgetMonthlySeasonal)
	seasonal := testAccBudgetLifecycleConfig(name, "  growth_per_period = 1\n  allow_growth = false\n  seasonal_amounts = "+budgetMonthlySeasonal)

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: growth,
				ConfigStateChecks: []statecheck.StateCheck{
					// Create turns growth on from the rate.
					budgetState("allow_growth", knownvalue.Bool(true)),
					budgetState("amount", knownvalue.Float64Exact(100)),
					budgetState("current_period_amount", knownvalue.NotNull()),
				},
			},
			budgetDriftStep(growth),
			{
				Config:      seasonalKeepsGrowth,
				PlanOnly:    true,
				ExpectError: budgetErrorPattern(`Set allow_growth = false to switch it to seasonal amounts`),
			},
			{
				Config:           seasonal,
				ConfigPlanChecks: budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("allow_growth", knownvalue.Bool(false)),
					budgetState("growth_per_period", knownvalue.Float64Exact(1)),
					budgetState("amount", knownvalue.Null()),
					budgetState("seasonal_amounts", knownvalue.ListSizeExact(12)),
				},
			},
			budgetDriftStep(seasonal),
		},
	})
}

// TestAccBudget_Rollover sets and removes rollover. The API returns a
// disabled rollover for every budget, which state keeps as null unless the
// configuration sets one; removing the block disables rollover.
func TestAccBudget_Rollover(t *testing.T) {
	name := fmt.Sprintf("test-rollover-%d", acctest.RandInt())
	without := testAccBudgetLifecycleConfig(name, "  amount = 100")
	with3 := testAccBudgetLifecycleConfig(name, "  amount = 100\n  rollover = { enabled = true, limit = 3 }")
	with2 := testAccBudgetLifecycleConfig(name, "  amount = 100\n  rollover = { enabled = true, limit = 2 }")
	withoutRenamed := testAccBudgetLifecycleConfig(name+"-renamed", "  amount = 100")
	withRenamedDescription := testAccBudgetLifecycleConfig(name+"-renamed", "  amount = 100\n  description = \"changed\"")
	var budgetID string

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders:        budgetTimeProvider(),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: with3,
				ConfigStateChecks: []statecheck.StateCheck{
					budgetState("rollover", knownvalue.ObjectExact(map[string]knownvalue.Check{
						"enabled": knownvalue.Bool(true),
						"limit":   knownvalue.Int64Exact(3),
					})),
				},
				Check: resource.TestCheckResourceAttrWith("doit_budget.this", "id", func(value string) error {
					budgetID = value
					return nil
				}),
			},
			budgetDriftStep(with3),
			// Removing the block plans null and disables rollover.
			{
				Config: without,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_budget.this", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue("doit_budget.this", tfjsonpath.New("rollover"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{budgetState("rollover", knownvalue.Null())},
			},
			budgetDriftStep(without),
			{
				Config:            with2,
				ConfigPlanChecks:  budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{budgetState("rollover", knownvalue.NotNull())},
			},
			// Removing it together with a rename.
			{
				Config:            withoutRenamed,
				ConfigPlanChecks:  budgetUpdateChecks(),
				ConfigStateChecks: []statecheck.StateCheck{budgetState("rollover", knownvalue.Null())},
			},
			budgetDriftStep(withoutRenamed),
			// An update of another attribute plans rollover as null, not
			// unknown, for a budget that does not use it.
			{
				Config: withRenamedDescription,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_budget.this", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue("doit_budget.this", tfjsonpath.New("rollover"), knownvalue.Null()),
						plancheck.ExpectUnknownValue("doit_budget.this", tfjsonpath.New("current_period_amount")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{budgetState("rollover", knownvalue.Null())},
			},
			// A rollover enabled outside Terraform is a change that the next
			// apply reverts, since the configuration does not set one.
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.UpdateBudgetWithBodyWithResponse(t.Context(), budgetID, "application/json",
						strings.NewReader(`{"rollover": {"enabled": true, "limit": 4}}`))
					if err != nil {
						t.Fatalf("enabling rollover via API: %v", err)
					}
					if resp.StatusCode() != 200 {
						t.Fatalf("enabling rollover via API: status %d: %s", resp.StatusCode(), resp.Body)
					}
				},
				Config: withRenamedDescription,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_budget.this", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue("doit_budget.this", tfjsonpath.New("rollover"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{budgetState("rollover", knownvalue.Null())},
			},
			budgetDriftStep(withRenamedDescription),
		},
	})
}

// TestAccBudget_SeasonalValidation checks the combinations the API rejects
// with seasonal amounts, and the rollover limits, at plan time. An unknown
// value defers the check to apply.
func TestAccBudget_SeasonalValidation(t *testing.T) {
	const seasonal = "seasonal_amounts = " + budgetMonthlySeasonal
	cases := []struct {
		name       string
		settings   string
		dependency string
		wantError  string
	}{
		{"amount", seasonal + "\n  amount = 100", "", `Attribute amount cannot be set together with seasonal_amounts`},
		{"use_prev_spend", seasonal + "\n  use_prev_spend = true", "", `use_prev_spend cannot be true together with seasonal_amounts`},
		{"allow_growth", seasonal + "\n  allow_growth = true", "", `allow_growth cannot be true together with seasonal_amounts`},
		{"growth rate on create", seasonal + "\n  growth_per_period = 5", "", `A growth_per_period above 0 turns growth on`},
		{"growth rate with allow_growth false", seasonal + "\n  growth_per_period = 5\n  allow_growth = false", "", ""},
		{"rollover enabled", seasonal + "\n  rollover = { enabled = true, limit = 2 }", "", `Rollover cannot be enabled together with seasonal_amounts`},
		{"rollover disabled", seasonal + "\n  rollover = { enabled = false, limit = 0 }", "", ""},
		{"wrong length", "seasonal_amounts = [1, 2, 3]", "", `time_interval "month" requires exactly 12 seasonal_amounts`},
		{"negative amount", "seasonal_amounts = [-1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]", "", `must be at least 0`},
		{"rollover limit 0", "amount = 100\n  rollover = { enabled = true, limit = 0 }", "", `rollover.limit must be at least 1`},
		{"rollover above maximum", "amount = 100\n  rollover = { enabled = true, limit = 13 }", "", `rollover.limit can be at most 12 for time_interval "month"`},
		{"unknown amount", seasonal + "\n  amount = terraform_data.dep.output", `resource "terraform_data" "dep" { input = 100 }`, ""},
		{"unknown seasonal amounts", "amount = 100\n  seasonal_amounts = terraform_data.dep.output", `resource "terraform_data" "dep" { input = [1, 2] }`, ""},
		{"unknown rollover limit", "amount = 100\n  rollover = { enabled = true, limit = terraform_data.dep.output }", `resource "terraform_data" "dep" { input = 99 }`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step := resource.TestStep{
				Config: fmt.Sprintf(`
%s
resource "doit_budget" "this" {
  name          = "test-budget-seasonal-validation"
  currency      = "USD"
  type          = "recurring"
  time_interval = "month"
  start_period  = 1759276800000
  %s
  scopes = [{
    id     = "allocation_rule"
    type   = "allocation_rule"
    mode   = "is"
    values = [%q]
  }]
}
`, tc.dependency, tc.settings, testAttribution()),
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
