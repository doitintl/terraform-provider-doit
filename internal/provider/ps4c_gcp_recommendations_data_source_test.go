package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpRecommendationsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpRecommendationsDataSourceConfig("", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendations.test", "billing_account_id"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendations.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendations.test", "row_count"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpRecommendationsDataSourceConfig("", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpRecommendationsDataSource_FilteredByService(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpRecommendationsDataSourceConfig("compute", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_recommendations.test", "gcp_service", "compute"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendations.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_recommendations.test", "row_count"),
				),
			},
			{
				Config: testAccPs4cGcpRecommendationsDataSourceConfig("compute", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpRecommendationsDataSource_NotFound(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccPs4cGcpRecommendationsDataSourceNotFoundConfig(),
				ExpectError: regexp.MustCompile(`(?i)not found|status 404`),
			},
		},
	})
}

func testAccPs4cGcpRecommendationsDataSourceConfig(service, region string) string {
	filterConfig := ""
	if service != "" {
		filterConfig += `
  gcp_service = "` + service + `"`
	}
	if region != "" {
		filterConfig += `
  region = "` + region + `"`
	}

	return `
data "doit_ps4c_gcp_billing_accounts" "list" {}

locals {
  accounts = data.doit_ps4c_gcp_billing_accounts.list.items
  first_billing_account_id = length(local.accounts) > 0 ? local.accounts[0].billing_account_id : "000000-000000-000000"
}

data "doit_ps4c_gcp_recommendations" "test" {
  billing_account_id = local.first_billing_account_id
` + filterConfig + `

  lifecycle {
    precondition {
      condition     = length(local.accounts) > 0
      error_message = "No PS4C GCP billing accounts returned; cannot run doit_ps4c_gcp_recommendations tests."
    }
  }
}
`
}

func testAccPs4cGcpRecommendationsDataSourceNotFoundConfig() string {
	return `
data "doit_ps4c_gcp_recommendations" "test" {
  billing_account_id = "000000-000000-000000"
}
`
}

func TestAccPs4cGcpRecommendationsDataSource_InvalidFilter(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_ps4c_gcp_recommendations" "test" {
  billing_account_id = "012345-6789AB-CDEF01"
  region             = "us_central1"
}
`,
				ExpectError: regexp.MustCompile(`(?s)region requires gcp_service to be specified`),
			},
			{
				Config: `
data "doit_ps4c_gcp_recommendations" "test" {
  billing_account_id = "012345-6789AB-CDEF01"
  gcp_service        = "compute"
  region             = "us_central1"
}
`,
				ExpectError: regexp.MustCompile(`(?s)Compute.*support global scope`),
			},
			{
				Config: `
data "doit_ps4c_gcp_recommendations" "test" {
  billing_account_id = "012345-6789AB-CDEF01"
  gcp_service        = "cloud_sql"
  region             = "global"
}
`,
				ExpectError: regexp.MustCompile(`(?s)Cloud SQL.*concrete GCP region`),
			},
		},
	})
}
