package provider_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
