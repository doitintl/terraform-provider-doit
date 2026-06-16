package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCloudconnectAwsAccountDataSource_Basic(t *testing.T) {
	accountID := os.Getenv("TEST_AWS_ACCOUNT_ID")
	roleArn := os.Getenv("TEST_AWS_ROLE_ARN")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudconnectAwsAccountDataSourceConfig(accountID, roleArn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.doit_cloudconnect_aws_account.test", "account_id",
						"doit_cloudconnect_aws_account.test", "account_id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_cloudconnect_aws_account.test", "role_arn",
						"doit_cloudconnect_aws_account.test", "role_arn"),
					resource.TestCheckResourceAttrPair(
						"data.doit_cloudconnect_aws_account.test", "time_linked",
						"doit_cloudconnect_aws_account.test", "time_linked"),
				),
			},
			// Drift check — re-apply same config, expect no changes.
			{
				Config: testAccCloudconnectAwsAccountDataSourceConfig(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccCloudconnectAwsAccountDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudconnectAwsAccountDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)error reading cloudconnect aws account|not found|404`),
			},
		},
	})
}

func testAccCloudconnectAwsAccountDataSourceConfig(accountID, roleArn string) string {
	return fmt.Sprintf(`
resource "doit_cloudconnect_aws_account" "test" {
  account_id       = %q
  role_arn         = %q
  enabled_features = []
}

data "doit_cloudconnect_aws_account" "test" {
  account_id = doit_cloudconnect_aws_account.test.account_id
}
`, accountID, roleArn)
}

func testAccCloudconnectAwsAccountDataSourceNotFoundConfig() string {
	return `
data "doit_cloudconnect_aws_account" "notfound" {
  account_id = "000000000000"
}
`
}
