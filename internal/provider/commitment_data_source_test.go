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

func testAccCommitmentDataSourceConfig(id string) string {
	return `
data "doit_commitment" "test" {
  id = "` + id + `"
}
`
}
