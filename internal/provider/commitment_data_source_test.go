package provider_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCommitmentDataSource_Basic(t *testing.T) {
	commitmentID := os.Getenv("TEST_COMMITMENT_ID")
	if commitmentID == "" {
		t.Skip("TEST_COMMITMENT_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentDataSourceConfig(commitmentID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_commitment.test", "id", commitmentID),
					resource.TestCheckResourceAttrSet("data.doit_commitment.test", "name"),
					resource.TestCheckResourceAttrSet("data.doit_commitment.test", "cloud_provider"),
					// Note: nullable float fields (total_commitment_value, total_current_attainment,
					// total_forecast_value, total_marketplace_spend) are tested via outputs in
					// TestAccCommitmentDataSource_PeriodAttributes to avoid flaky assertions
					// when the API omits these optional pointer fields.
				),
			},
			// Drift verification: re-apply should produce no changes
			{
				Config:   testAccCommitmentDataSourceConfig(commitmentID),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccCommitmentDataSourceConfig(commitmentID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccCommitmentDataSource_PeriodAttributes verifies that the new period-level
// attributes (forecast_value, marketplace_limit_amount, marketplace_spend) are
// accessible via Terraform outputs and don't cause drift.
func TestAccCommitmentDataSource_PeriodAttributes(t *testing.T) {
	commitmentID := os.Getenv("TEST_COMMITMENT_ID")
	if commitmentID == "" {
		t.Skip("TEST_COMMITMENT_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCommitmentDataSourcePeriodAttributesConfig(commitmentID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_commitment.test", "periods.#"),
					// Verify new period-level fields are accessible via output
					resource.TestCheckOutput("has_periods", "true"),
				),
			},
			// Drift verification
			{
				Config: testAccCommitmentDataSourcePeriodAttributesConfig(commitmentID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCommitmentDataSourceConfig(id string) string {
	return `
data "doit_commitment" "test" {
  id = "` + id + `"
}
`
}

func testAccCommitmentDataSourcePeriodAttributesConfig(id string) string {
	return `
data "doit_commitment" "test" {
  id = "` + id + `"
}

output "has_periods" {
  value = length(data.doit_commitment.test.periods) > 0
}

# Exercise top-level nullable float fields (may be null if API omits them)
output "total_commitment_value" {
  value = data.doit_commitment.test.total_commitment_value
}

output "total_current_attainment" {
  value = data.doit_commitment.test.total_current_attainment
}

output "total_forecast_value" {
  value = data.doit_commitment.test.total_forecast_value
}

output "total_marketplace_spend" {
  value = data.doit_commitment.test.total_marketplace_spend
}

# Exercise new period-level attributes to ensure they are populated without error
output "first_period_forecast_value" {
  value = length(data.doit_commitment.test.periods) > 0 ? data.doit_commitment.test.periods[0].forecast_value : null
}

output "first_period_marketplace_spend" {
  value = length(data.doit_commitment.test.periods) > 0 ? data.doit_commitment.test.periods[0].marketplace_spend : null
}

output "first_period_marketplace_limit_amount" {
  value = length(data.doit_commitment.test.periods) > 0 ? data.doit_commitment.test.periods[0].marketplace_limit_amount : null
}
`
}
