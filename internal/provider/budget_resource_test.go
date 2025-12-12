package provider_test

import (
	"fmt"
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
			},
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
						tfjsonpath.New("description"),
						knownvalue.StringExact("test budget update")),
				},
			},
		},
	})
}

func budgetStartPeriod() string {
	return `
locals {
  start_period = provider::time::rfc3339_parse("2025-11-01T00:00:00Z").unix * 1000
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
  name = "test-%d"
  description = "test budget"
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
  amount            = 200
  currency          = "EUR"
  growth_per_period = 10
  time_interval     = "month"
  type              = "recurring"
  use_prev_spend    = false
  start_period = local.start_period
}
`, budgetStartPeriod(), i)
}

func testAccBudgetUpdate(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name = "test-%d"
  description = "test budget update"
  alerts = [
    {
      percentage = 60
    },
    {
      percentage = 90,
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
  amount            = 500
  currency          = "EUR"
  growth_per_period = 10
  time_interval     = "month"
  type              = "recurring"
  use_prev_spend    = true
  start_period = local.start_period
}
`, budgetStartPeriod(), i)
}
