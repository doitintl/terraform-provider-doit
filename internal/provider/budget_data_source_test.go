package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBudgetDataSource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-budget-ds")

	resource.ParallelTest(t, resource.TestCase{
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
				Config: testAccBudgetDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source attributes match resource
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("test budget for data source")),
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("amount"),
						knownvalue.Float64Exact(500)),
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("currency"),
						knownvalue.StringExact("EUR")),
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("recurring")),
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("time_interval"),
						knownvalue.StringExact("month")),
					// Verify alerts list is populated
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("alerts"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccBudgetDataSourceConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccBudgetDataSource_WithScopes(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-budget-ds-s")

	resource.ParallelTest(t, resource.TestCase{
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
				Config: testAccBudgetDataSourceWithScopesConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("scopes"),
						knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("scopes").AtSliceIndex(0).AtMapKey("id"),
						knownvalue.StringExact("cloud_provider")),
				},
			},
		},
	})
}

func testAccBudgetDataSourceConfig(name string) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "test" {
  name          = %q
  description   = "test budget for data source"
  amount        = 500
  currency      = "EUR"
  time_interval = "month"
  scope         = ["%s"]
  alerts = [
    { percentage = 50 },
    { percentage = 80 }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}

data "doit_budget" "test" {
    id = doit_budget.test.id
}
`, budgetStartPeriod(), name, testAttribution(), testUser())
}

func testAccBudgetDataSourceWithScopesConfig(name string) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "test" {
  name          = %q
  amount        = 200
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
    { percentage = 75 }
  ]
  collaborators = [
    {
      "email" : "%s",
      "role" : "owner"
    },
  ]
  type          = "recurring"
  start_period  = local.start_period
}

data "doit_budget" "test" {
    id = doit_budget.test.id
}
`, budgetStartPeriod(), name, testUser())
}

func TestAccBudgetDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccBudgetDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(not found|404|Not Found)`),
			},
		},
	})
}

func testAccBudgetDataSourceNotFoundConfig() string {
	return `
data "doit_budget" "test" {
    id = "non-existent-budget-id"
}
`
}
