package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBudget(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Test Budget (Recurring)
			{
				Config: testAccBudget(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_budget.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-min-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("recurring")),
				},
			},
			// Test Budget Update (In-place)
			{
				Config: testAccBudgetUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_budget.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("amount"),
						knownvalue.Float64Exact(150)),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-min-updated-%d", n))),
				},
			},
			// Test Fixed Budget
			{
				Config: testAccBudgetFixed(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("type"),
						knownvalue.StringExact("fixed")),
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("end_period"),
						knownvalue.NotNull()),
				},
			},
			// Test Full Budget (Recurring)
			{
				Config: testAccBudgetFull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
		},
	})
}

func budgetStartPeriod() string {
	return `
locals {
  start_period = provider::time::rfc3339_parse("2025-10-01T00:00:00Z").unix * 1000
}

output "start_period" {
	value = local.start_period
}
`
}

func testAccBudget(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-min-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scope         = ["ydDBFKVuz9kGlFDex8cN"]
  alerts = [
    { percentage = 50 }
  ]
  collaborators = [
    {
      "email" : "hannes.h@doit.com",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i)
}

func testAccBudgetUpdate(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-min-updated-%d"
  amount        = 150
  currency      = "EUR"
  time_interval = "month"
  scope         = ["ydDBFKVuz9kGlFDex8cN"]
  alerts = [
    { percentage = 50 },
    { percentage = 80 }
  ]
  collaborators = [
    {
      "email" : "hannes.h@doit.com",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i)
}

func testAccBudgetFixed(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-fixed-%d"
  amount        = 500
  type          = "fixed"
  start_period  = local.start_period
  end_period    = local.start_period + (30 * 24 * 60 * 60 * 1000) # 30 days later
  scope         = ["ydDBFKVuz9kGlFDex8cN"] # Required by validator
}
`, budgetStartPeriod(), i)
}

func testAccBudgetFull(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name = "test-full-%d"
  description = "test full budget"
  alerts = [
    {
      percentage = 50
    },
    {
      percentage = 85,
    },
    {
      percentage = 100,
    }
  ]
  recipients = [
    "hannes.h@doit.com"
  ]
  collaborators = [
    {
      "email" : "hannes.h@doit.com",
      "role" : "owner"
    },
  ]
  scope = [
    "ydDBFKVuz9kGlFDex8cN"
  ]
  amount            = 1000
  currency          = "EUR"
  growth_per_period = 5
  time_interval     = "quarter"
  type              = "recurring"
  use_prev_spend    = false
  start_period      = local.start_period
  public            = "viewer"
}
`, budgetStartPeriod(), i)
}

func TestAccBudget_Import(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudget(n),
			},
			{
				ResourceName:      "doit_budget.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccBudget_Scopes(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetScopes(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_budget.this",
						tfjsonpath.New("scopes"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("cloud_provider"),
								"mode":   knownvalue.StringExact("is"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("amazon-web-services")}),
							}),
						},
						),
					),
				},
			},
		},
	})
}

func TestAccBudget_Conflict(t *testing.T) {
	n := rand.Int()

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13.1",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccBudgetConflict(n),
				ExpectError: regexp.MustCompile("Attributes 'scope' and 'scopes' are mutually exclusive"),
			},
		},
	})
}

func testAccBudgetScopes(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-scopes-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  scopes = [
    {
      type   = "fixed"
      id     = "cloud_provider"
      mode   = "is"
      values = ["amazon-web-services"]
    }
  ]
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  collaborators = [
    {
      "email" : "hannes.h@doit.com",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i)
}

func testAccBudgetConflict(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-conflict-%d"
  amount        = 100
  scope         = ["ydDBFKVuz9kGlFDex8cN"]
  scopes = [
    {
      type   = "attribution"
      id     = "attribution"
      mode   = "is"
      values = ["ydDBFKVuz9kGlFDex8cN"]
    }
  ]
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  collaborators = [
    {
      "email" : "hannes.h@doit.com",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}
`, budgetStartPeriod(), i)
}
