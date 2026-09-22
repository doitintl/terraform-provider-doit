package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpPlannedPurchasesDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourceConfig("", "", 0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "billing_account_id"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "row_count"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourceConfig("", "", 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpPlannedPurchasesDataSource_WithGcpService(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourceConfig("compute", "", 0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_planned_purchases.test", "gcp_service", "compute"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "row_count"),
				),
			},
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourceConfig("compute", "", 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpPlannedPurchasesDataSource_WithGcpServiceAndRegion(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourceConfig("compute", "global", 0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_planned_purchases.test", "gcp_service", "compute"),
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_planned_purchases.test", "region", "global"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.test", "row_count"),
				),
			},
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourceConfig("compute", "global", 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpPlannedPurchasesDataSource_Pagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourcePaginationConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_planned_purchases.first_page", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_planned_purchases.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_planned_purchases.paginated", "items.#", "1"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_ps4c_gcp_planned_purchases.first_page", "items.0.service",
						"data.doit_ps4c_gcp_planned_purchases.paginated", "items.0.service"),
				),
			},
			{
				Config: testAccPs4cGcpPlannedPurchasesDataSourcePaginationConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpPlannedPurchasesDataSource_ValidationErrors(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_ps4c_gcp_planned_purchases" "invalid" {
  billing_account_id = "000000-000000-000000"
  region             = "global"
}
`,
				ExpectError: regexp.MustCompile(`Attribute "region" requires "gcp_service"`),
			},
			{
				Config: `
data "doit_ps4c_gcp_planned_purchases" "invalid" {
  billing_account_id = "000000-000000-000000"
  gcp_service        = "compute"
  region             = "us_central1"
}
`,
				ExpectError: regexp.MustCompile(`When "gcp_service" is "compute", "region" must be "global"`),
			},
			{
				Config: `
data "doit_ps4c_gcp_planned_purchases" "invalid" {
  billing_account_id = "000000-000000-000000"
  gcp_service        = "cloud_sql"
  region             = "global"
}
`,
				ExpectError: regexp.MustCompile(`When "gcp_service" is "cloud_sql", "region" must be a concrete GCP region`),
			},
			{
				Config: `
data "doit_ps4c_gcp_planned_purchases" "invalid" {
  billing_account_id = "000000-000000-000000"
  page_token         = "cursor-123"
}
`,
				ExpectError: regexp.MustCompile(`Attribute "page_token" requires "max_results"`),
			},
		},
	})
}

func testAccPs4cGcpPlannedPurchasesDataSourceConfig(gcpService, region string, maxResults int) string {
	var extraConfig string
	if gcpService != "" {
		extraConfig += fmt.Sprintf("\n  gcp_service = %q", gcpService)
	}
	if region != "" {
		extraConfig += fmt.Sprintf("\n  region      = %q", region)
	}
	if maxResults > 0 {
		extraConfig += fmt.Sprintf("\n  max_results = %d", maxResults)
	}

	return fmt.Sprintf(`
data "doit_ps4c_gcp_billing_accounts" "list" {}

locals {
  accounts = data.doit_ps4c_gcp_billing_accounts.list.items
  first_billing_account_id = length(local.accounts) > 0 ? local.accounts[0].billing_account_id : "000000-000000-000000"
}

data "doit_ps4c_gcp_planned_purchases" "test" {
  billing_account_id = local.first_billing_account_id%s

  lifecycle {
    precondition {
      condition     = length(local.accounts) > 0
      error_message = "No PS4C GCP billing accounts returned; cannot run doit_ps4c_gcp_planned_purchases tests."
    }
  }
}
`, extraConfig)
}

func testAccPs4cGcpPlannedPurchasesDataSourcePaginationConfig() string {
	return `
data "doit_ps4c_gcp_billing_accounts" "list" {}

locals {
  accounts = data.doit_ps4c_gcp_billing_accounts.list.items
  first_billing_account_id = length(local.accounts) > 0 ? local.accounts[0].billing_account_id : "000000-000000-000000"
}

data "doit_ps4c_gcp_planned_purchases" "first_page" {
  billing_account_id = local.first_billing_account_id
  max_results        = 1

  lifecycle {
    precondition {
      condition     = length(local.accounts) > 0
      error_message = "No PS4C GCP billing accounts returned; cannot run doit_ps4c_gcp_planned_purchases tests."
    }
  }
}

data "doit_ps4c_gcp_planned_purchases" "paginated" {
  billing_account_id = local.first_billing_account_id
  max_results        = 1
  page_token         = data.doit_ps4c_gcp_planned_purchases.first_page.page_token
}
`
}
