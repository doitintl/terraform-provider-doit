package provider_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccBudget_DataSourceInvalid verifies the enum validator rejects a value
// that is valid for reports but not for budgets.
func TestAccBudget_DataSourceInvalid(t *testing.T) {
	n := acctest.RandInt()

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
			{
				Config:      testAccBudgetDataSource(n, "bqlens"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`data_source`),
			},
		},
	})
}

// TestAccBudget_DataSourcePreservedOnUpdate verifies that an update changing
// another field, with data_source omitted, keeps the saved data source.
func TestAccBudget_DataSourcePreservedOnUpdate(t *testing.T) {
	n := acctest.RandInt()
	cfg := testAccBudgetDataSource(n, "")
	updated := strings.Replace(cfg, "amount        = 100", "amount        = 200", 1)
	path := tfjsonpath.New("data_source")

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
			dataSourceStep(cfg, "doit_budget.this", path, billingDataSourceResolved),
			{
				Config: updated,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_budget.this", plancheck.ResourceActionUpdate),
					},
				},
			},
			dataSourceStep(updated, "doit_budget.this", path, billingDataSourceResolved),
			driftStep(updated),
		},
	})
}
