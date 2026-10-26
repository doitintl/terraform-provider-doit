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

// When data_source is omitted on create the API resolves it to billing-datahub
// for customers with DataHub metrics and to billing otherwise. These tests are
// tenant-agnostic: they assert the resolved value is one of the two and that it
// is stable, never which one.
var billingDataSourceResolved = knownvalue.StringRegexp(regexp.MustCompile(`^billing(-datahub)?$`))

// dataSourceStep returns a step applying cfg and checking the resolved
// data_source of addr at path, followed by the caller's drift check.
func dataSourceStep(cfg, addr string, path tfjsonpath.Path, want knownvalue.Check) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, path, want),
		},
	}
}

func driftStep(cfg string) resource.TestStep {
	return resource.TestStep{
		Config: cfg,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

func TestAccAlert_DataSourceDefault(t *testing.T) {
	n := acctest.RandInt()
	path := tfjsonpath.New("config").AtMapKey("data_source")
	const addr = "doit_alert.this"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Omitted on create: the API resolves it, so the plan holds Unknown.
			dataSourceStep(testAccAlertDataSource(n, ""), addr, path, billingDataSourceResolved),
			driftStep(testAccAlertDataSource(n, "")),
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Explicit values round-trip in both directions.
			dataSourceStep(testAccAlertDataSource(n, "billing"), addr, path, knownvalue.StringExact("billing")),
			driftStep(testAccAlertDataSource(n, "billing")),
			dataSourceStep(testAccAlertDataSource(n, "billing-datahub"), addr, path, knownvalue.StringExact("billing-datahub")),
			driftStep(testAccAlertDataSource(n, "billing-datahub")),
			// Removing the attribute keeps the saved value (not clearable).
			dataSourceStep(testAccAlertDataSource(n, ""), addr, path, knownvalue.StringExact("billing-datahub")),
			driftStep(testAccAlertDataSource(n, "")),
		},
	})
}

func testAccAlertDataSource(i int, dataSource string) string {
	line := ""
	if dataSource != "" {
		line = fmt.Sprintf("data_source   = %q", dataSource)
	}
	return fmt.Sprintf(`
resource "doit_alert" "this" {
  name = "test-alert-ds-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    %s
  }
}
`, i, line)
}

func TestAccBudget_DataSourceDefault(t *testing.T) {
	n := acctest.RandInt()
	path := tfjsonpath.New("data_source")
	const addr = "doit_budget.this"

	resource.ParallelTest(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: "~> 0.13",
			},
		},
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			dataSourceStep(testAccBudgetDataSource(n, ""), addr, path, billingDataSourceResolved),
			driftStep(testAccBudgetDataSource(n, "")),
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"scopes.0.id", "scopes.0.type"},
			},
			dataSourceStep(testAccBudgetDataSource(n, "billing"), addr, path, knownvalue.StringExact("billing")),
			driftStep(testAccBudgetDataSource(n, "billing")),
			dataSourceStep(testAccBudgetDataSource(n, "billing-datahub"), addr, path, knownvalue.StringExact("billing-datahub")),
			driftStep(testAccBudgetDataSource(n, "billing-datahub")),
			// Removing the attribute keeps the saved value (not clearable).
			dataSourceStep(testAccBudgetDataSource(n, ""), addr, path, knownvalue.StringExact("billing-datahub")),
			driftStep(testAccBudgetDataSource(n, "")),
		},
	})
}

func testAccBudgetDataSource(i int, dataSource string) string {
	line := ""
	if dataSource != "" {
		line = fmt.Sprintf("data_source   = %q", dataSource)
	}
	return fmt.Sprintf(`
%s

resource "doit_budget" "this" {
  name          = "test-budget-ds-%d"
  amount        = 100
  currency      = "EUR"
  time_interval = "month"
  type          = "recurring"
  start_period  = local.start_period
  %s
  scopes = [
    {
      type   = "allocation_rule"
      id     = "allocation_rule"
      mode   = "is"
      values = ["%s"]
    }
  ]
  collaborators = [
    {
      email = "%s"
      role  = "owner"
    },
  ]
}
`, budgetStartPeriod(), i, line, testAttribution(), testUser())
}

func TestAccReport_DataSourceDefault(t *testing.T) {
	n := acctest.RandInt()
	path := tfjsonpath.New("config").AtMapKey("data_source")
	const addr = "doit_report.this"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			dataSourceStep(testAccReportDefaultDataSource(n, ""), addr, path, billingDataSourceResolved),
			driftStep(testAccReportDefaultDataSource(n, "")),
			// Removing an explicit value keeps the saved one (PATCH omission preserves it).
			dataSourceStep(testAccReportDefaultDataSource(n, "billing"), addr, path, knownvalue.StringExact("billing")),
			driftStep(testAccReportDefaultDataSource(n, "billing")),
			dataSourceStep(testAccReportDefaultDataSource(n, ""), addr, path, knownvalue.StringExact("billing")),
			driftStep(testAccReportDefaultDataSource(n, "")),
		},
	})
}

func testAccReportDefaultDataSource(i int, dataSource string) string {
	line := ""
	if dataSource != "" {
		line = fmt.Sprintf("data_source    = %q", dataSource)
	}
	return fmt.Sprintf(`
resource "doit_report" "this" {
  name = "test-report-ds-%d"
  config = {
    metrics = [{
      type  = "basic"
      value = "cost"
    }]
    aggregation    = "total"
    time_interval  = "month"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
    %s
  }
}
`, i, line)
}
