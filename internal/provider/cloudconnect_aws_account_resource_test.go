package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccCloudconnectAwsAccount_preCheck skips tests when the required
// CloudConnect AWS environment variables are not set.
func testAccCloudconnectAwsAccount_preCheck(t *testing.T) {
	t.Helper()
	testAccPreCheckFunc(t)()
	if os.Getenv("TEST_AWS_ACCOUNT_ID") == "" {
		t.Skip("TEST_AWS_ACCOUNT_ID not set")
	}
	if os.Getenv("TEST_AWS_ROLE_ARN") == "" {
		t.Skip("TEST_AWS_ROLE_ARN not set")
	}
}

func testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn string) string {
	return fmt.Sprintf(`
resource "doit_cloudconnect_aws_account" "this" {
  account_id       = %q
  role_arn         = %q
  enabled_features = []
}
`, accountID, roleArn)
}

func testAccCloudconnectAwsAccountConfig_withBucket(accountID, roleArn, bucket, region string) string {
	return fmt.Sprintf(`
resource "doit_cloudconnect_aws_account" "this" {
  account_id       = %q
  role_arn         = %q
  s3bucket         = %q
  s3bucket_region  = %q
  enabled_features = ["real-time-data"]
}
`, accountID, roleArn, bucket, region)
}

func TestAccCloudconnectAwsAccount_Basic(t *testing.T) {
	accountID := os.Getenv("TEST_AWS_ACCOUNT_ID")
	roleArn := os.Getenv("TEST_AWS_ROLE_ARN")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with just account_id + role_arn
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_cloudconnect_aws_account.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("account_id"),
						knownvalue.StringExact(accountID)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("role_arn"),
						knownvalue.StringExact(roleArn)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("supported_features"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("time_linked"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift check — re-apply same config, expect no changes.
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccCloudconnectAwsAccount_WithS3Bucket(t *testing.T) {
	accountID := os.Getenv("TEST_AWS_ACCOUNT_ID")
	roleArn := os.Getenv("TEST_AWS_ROLE_ARN")
	bucket := os.Getenv("TEST_AWS_S3_BUCKET")
	bucketRegion := os.Getenv("TEST_AWS_S3_BUCKET_REGION")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
			if os.Getenv("TEST_AWS_S3_BUCKET") == "" {
				t.Skip("TEST_AWS_S3_BUCKET not set")
			}
			if os.Getenv("TEST_AWS_S3_BUCKET_REGION") == "" {
				t.Skip("TEST_AWS_S3_BUCKET_REGION not set")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with all fields including bucket
			{
				Config: testAccCloudconnectAwsAccountConfig_withBucket(accountID, roleArn, bucket, bucketRegion),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_cloudconnect_aws_account.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("account_id"),
						knownvalue.StringExact(accountID)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("role_arn"),
						knownvalue.StringExact(roleArn)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("s3bucket"),
						knownvalue.StringExact(bucket)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("s3bucket_region"),
						knownvalue.StringExact(bucketRegion)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("supported_features"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("time_linked"),
						knownvalue.NotNull()),
				},
			},
			// Step 2: Drift check — re-apply same config, expect no changes.
			{
				Config: testAccCloudconnectAwsAccountConfig_withBucket(accountID, roleArn, bucket, bucketRegion),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccCloudconnectAwsAccount_Update(t *testing.T) {
	accountID := os.Getenv("TEST_AWS_ACCOUNT_ID")
	roleArn := os.Getenv("TEST_AWS_ROLE_ARN")
	bucket := os.Getenv("TEST_AWS_S3_BUCKET")
	bucketRegion := os.Getenv("TEST_AWS_S3_BUCKET_REGION")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
			if os.Getenv("TEST_AWS_S3_BUCKET") == "" {
				t.Skip("TEST_AWS_S3_BUCKET not set")
			}
			if os.Getenv("TEST_AWS_S3_BUCKET_REGION") == "" {
				t.Skip("TEST_AWS_S3_BUCKET_REGION not set")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with bucket
			{
				Config: testAccCloudconnectAwsAccountConfig_withBucket(accountID, roleArn, bucket, bucketRegion),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_cloudconnect_aws_account.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("s3bucket"),
						knownvalue.StringExact(bucket)),
					statecheck.ExpectKnownValue(
						"doit_cloudconnect_aws_account.this",
						tfjsonpath.New("s3bucket_region"),
						knownvalue.StringExact(bucketRegion)),
				},
			},
			// Step 2: Update to remove s3bucket + s3bucket_region
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_cloudconnect_aws_account.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Step 3: Drift check — re-apply same config, expect no changes.
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccCloudconnectAwsAccount_Import(t *testing.T) {
	accountID := os.Getenv("TEST_AWS_ACCOUNT_ID")
	roleArn := os.Getenv("TEST_AWS_ROLE_ARN")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
			},
			// Step 2: Import
			{
				ResourceName:                         "doit_cloudconnect_aws_account.this",
				ImportState:                          true,
				ImportStateId:                        accountID,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "account_id",
			},
			// Step 3: Drift check — re-apply config after import, expect no changes.
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccCloudconnectAwsAccount_Disappears verifies that Terraform correctly
// handles resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccCloudconnectAwsAccount_Disappears(t *testing.T) {
	accountID := os.Getenv("TEST_AWS_ACCOUNT_ID")
	roleArn := os.Getenv("TEST_AWS_ROLE_ARN")

	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_AWS_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectAwsAccount_preCheck(t)
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and verify account_id
			{
				Config: testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"doit_cloudconnect_aws_account.this",
						"account_id",
						accountID,
					),
				),
			},
			// Step 2: Delete the resource via API, then verify Terraform detects the drift
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteAccountRoleWithResponse(context.Background(), accountID)
					if err != nil {
						t.Fatalf("Failed to delete CloudConnect AWS account via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204 or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccCloudconnectAwsAccountConfig_basic(accountID, roleArn),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}
