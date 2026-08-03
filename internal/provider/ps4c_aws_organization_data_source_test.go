package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccPs4cAwsOrganizationDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Chain: list organizations -> pick the first -> get it by ID.
				// No dedicated test env var; the test tenant's only PS4C AWS
				// organization is discovered live instead.
				Config: testAccPs4cAwsOrganizationDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_aws_organization.test", "management_account_id"),
					resource.TestCheckResourceAttrPair(
						"data.doit_ps4c_aws_organization.test", "management_account_id",
						"data.doit_ps4c_aws_organizations.list", "items.0.management_account_id",
					),
				),
				// The test tenant's only organization is not onboarded to PS4C,
				// so every onboarding-dependent nested object is expected to
				// resolve to null rather than erroring — this is the one
				// live-known fact about the test data and directly exercises
				// the nil-handling path in the mapping code.
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_ps4c_aws_organization.test", tfjsonpath.New("onboarding_status"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_ps4c_aws_organization.test", tfjsonpath.New("stats30d"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_ps4c_aws_organization.test", tfjsonpath.New("savings_totals"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_ps4c_aws_organization.test", tfjsonpath.New("monthly_potential_savings"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_ps4c_aws_organization.test", tfjsonpath.New("monthly_stats"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_ps4c_aws_organization.test", tfjsonpath.New("daily_coverage"), knownvalue.Null()),
				},
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cAwsOrganizationDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cAwsOrganizationDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccPs4cAwsOrganizationDataSourceConfigDirect("999999999999"),
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

func testAccPs4cAwsOrganizationDataSourceConfig() string {
	return `
data "doit_ps4c_aws_organizations" "list" {}

# Guard: fail with a clear message if the tenant has no PS4C AWS organizations,
# rather than an opaque index-out-of-range error on an empty list.
check "has_organizations" {
  assert {
    condition     = length(data.doit_ps4c_aws_organizations.list.items) > 0
    error_message = "No PS4C AWS organizations returned; cannot run doit_ps4c_aws_organization tests."
  }
}

locals {
  first_org = tolist(data.doit_ps4c_aws_organizations.list.items)[0]
}

data "doit_ps4c_aws_organization" "test" {
  management_account_id = local.first_org.management_account_id
}
`
}

func testAccPs4cAwsOrganizationDataSourceConfigDirect(managementAccountID string) string {
	return `
data "doit_ps4c_aws_organization" "test" {
  management_account_id = "` + managementAccountID + `"
}
`
}
