package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPs4cGcpSpendCudsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("", "", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "billing_account_id"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "row_count"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("", "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpSpendCudsDataSource_ActiveStatus(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("active", "", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_spend_cuds.test", "status", "active"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "row_count"),
				),
			},
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("active", "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpSpendCudsDataSource_FilterGcpService(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("", "compute", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_spend_cuds.test", "gcp_service", "compute"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "row_count"),
				),
			},
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("", "compute", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccPs4cGcpSpendCudsDataSource_FilterRegion(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("", "compute", "global"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_spend_cuds.test", "gcp_service", "compute"),
					resource.TestCheckResourceAttr("data.doit_ps4c_gcp_spend_cuds.test", "region", "global"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "items.#"),
					resource.TestCheckResourceAttrSet("data.doit_ps4c_gcp_spend_cuds.test", "row_count"),
				),
			},
			{
				Config: testAccPs4cGcpSpendCudsDataSourceConfig("", "compute", "global"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccPs4cGcpSpendCudsDataSourceConfig(status, gcpService, region string) string {
	extraConfig := ""
	if status != "" {
		extraConfig += "\n  status = \"" + status + "\""
	}
	if gcpService != "" {
		extraConfig += "\n  gcp_service = \"" + gcpService + "\""
	}
	if region != "" {
		extraConfig += "\n  region = \"" + region + "\""
	}

	return `
data "doit_ps4c_gcp_billing_accounts" "list" {}

locals {
  accounts = data.doit_ps4c_gcp_billing_accounts.list.items
  first_billing_account_id = length(local.accounts) > 0 ? local.accounts[0].billing_account_id : "000000-000000-000000"
}

data "doit_ps4c_gcp_spend_cuds" "test" {
  billing_account_id = local.first_billing_account_id` + extraConfig + `

  lifecycle {
    precondition {
      condition     = length(local.accounts) > 0
      error_message = "No PS4C GCP billing accounts returned; cannot run doit_ps4c_gcp_spend_cuds tests."
    }
  }
}
`
}
