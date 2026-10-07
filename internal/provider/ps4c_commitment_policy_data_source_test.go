package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cCommitmentPolicyDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cCommitmentPolicyDataSourceConfig("balanced"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "id", "balanced"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "policy_id", "balanced"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "name", "Balanced"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "is_built_in", "true"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "target_coverage", "80"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "lookback_days", "60"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "ladder_step_percent", "10"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "ladder_interval_days", "7"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policy.test", "bootstrap_percent", "35"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policy.test", "assignments.#"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cCommitmentPolicyDataSourceConfig("balanced"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cCommitmentPolicyDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccPs4cCommitmentPolicyDataSourceConfig("nonexistent_policy_12345"),
				ExpectError: regexp.MustCompile(`(?i)not found|status 404`),
			},
		},
	})
}

func testAccPs4cCommitmentPolicyDataSourceConfig(policyId string) string {
	return `
data "doit_ps4c_commitment_policy" "test" {
  policy_id = "` + policyId + `"
}
`
}
