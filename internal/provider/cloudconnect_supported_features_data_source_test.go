package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccCloudconnectSupportedFeaturesDataSource_Basic(t *testing.T) {
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
				Config: testAccCloudconnectSupportedFeaturesDataSourceConfig(accountID, roleArn),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_cloudconnect_supported_features.test",
						tfjsonpath.New("account_id"),
						knownvalue.StringExact(accountID),
					),
					statecheck.ExpectKnownValue(
						"data.doit_cloudconnect_supported_features.test",
						tfjsonpath.New("supported_features"),
						knownvalue.NotNull(),
					),
					statecheck.CompareValuePairs(
						"data.doit_cloudconnect_supported_features.test", tfjsonpath.New("supported_features"),
						"doit_cloudconnect_aws_account.test", tfjsonpath.New("supported_features"),
						compare.ValuesSame(),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"data.doit_cloudconnect_supported_features.test", "account_id", accountID,
					),
					resource.TestCheckResourceAttrSet(
						"data.doit_cloudconnect_supported_features.test", "supported_features.#",
					),
				),
			},
			{
				Config: testAccCloudconnectSupportedFeaturesDataSourceConfig(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccCloudconnectSupportedFeaturesDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{{
			Config:      `data "doit_cloudconnect_supported_features" "missing" { account_id = "000000000000" }`,
			ExpectError: regexp.MustCompile(`status: 404`),
		}},
	})
}

func testAccCloudconnectSupportedFeaturesDataSourceConfig(accountID, roleArn string) string {
	return fmt.Sprintf(`
resource "doit_cloudconnect_aws_account" "test" {
  account_id       = %q
  role_arn         = %q
  enabled_features = []
}

data "doit_cloudconnect_supported_features" "test" {
  account_id = doit_cloudconnect_aws_account.test.account_id
}
`, accountID, roleArn)
}
