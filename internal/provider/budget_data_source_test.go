package provider_test

import (
	"fmt"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBudgetDataSource(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

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
				Config: testAccBudgetDataSourceConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source attributes match resource
					statecheck.ExpectKnownValue(
						"data.doit_budget.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-ds-budget-%d", n))),
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
		},
	})
}

func TestAccBudgetDataSource_WithScopes(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

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
				Config: testAccBudgetDataSourceWithScopesConfig(n),
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

func testAccBudgetDataSourceConfig(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "test" {
  name          = "test-ds-budget-%d"
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
`, budgetStartPeriod(), i, testAttribution(), testUser())
}

func testAccBudgetDataSourceWithScopesConfig(i int) string {
	return fmt.Sprintf(`
%s

resource "doit_budget" "test" {
  name          = "test-ds-budget-scopes-%d"
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
`, budgetStartPeriod(), i, testUser())
}
