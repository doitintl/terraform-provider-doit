package provider_test

import (
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cCommitmentPoliciesDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cCommitmentPoliciesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.test", "row_count"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cCommitmentPoliciesDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cCommitmentPoliciesDataSource_Pagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cCommitmentPoliciesDataSourceConfigPagination(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policies.test", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.test", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.test", "row_count"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cCommitmentPoliciesDataSourceConfigPagination(1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cCommitmentPoliciesDataSource_PageTokenOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_ps4c_commitment_policies" "first_page" {
  max_results = 1
}

data "doit_ps4c_commitment_policies" "from_token" {
  page_token = data.doit_ps4c_commitment_policies.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policies.first_page", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.first_page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_ps4c_commitment_policies.from_token", "page_token"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_ps4c_commitment_policies.first_page", "items.0.id",
						"data.doit_ps4c_commitment_policies.from_token", "items.0.id"),
				),
			},
		},
	})
}

func TestAccPs4cCommitmentPoliciesDataSource_MaxResultsAndPageToken(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_ps4c_commitment_policies" "first_page" {
  max_results = 1
}

data "doit_ps4c_commitment_policies" "paginated" {
  max_results = 1
  page_token  = data.doit_ps4c_commitment_policies.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policies.first_page", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_ps4c_commitment_policies.paginated", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_commitment_policies.paginated", "page_token"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_ps4c_commitment_policies.first_page", "items.0.id",
						"data.doit_ps4c_commitment_policies.paginated", "items.0.id"),
				),
			},
		},
	})
}

func testAccPs4cCommitmentPoliciesDataSourceConfig() string {
	return `
data "doit_ps4c_commitment_policies" "test" {}
`
}

func testAccPs4cCommitmentPoliciesDataSourceConfigPagination(maxResults int) string {
	return `
data "doit_ps4c_commitment_policies" "test" {
  max_results = ` + strconv.Itoa(maxResults) + `
}
`
}
