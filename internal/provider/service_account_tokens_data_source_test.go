package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccServiceAccountTokensDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sats-ds")
	config := testAccServiceAccountTokensDataSourceConfig(rName)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("token_count", "2"),
					resource.TestCheckOutput("tokens_match", "true"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccServiceAccountTokensDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "doit_service_account" "this" {
  name = %[1]q
}

resource "doit_service_account_token" "first" {
  service_account_id = doit_service_account.this.id
  name               = "%[1]s-first"
  expires_time       = "2099-01-01T00:00:00Z"
}

resource "doit_service_account_token" "second" {
  service_account_id = doit_service_account.this.id
  name               = "%[1]s-second"
  state              = "disabled"
}

data "doit_service_account_tokens" "all" {
  service_account_id = doit_service_account.this.id

  depends_on = [
    doit_service_account_token.first,
    doit_service_account_token.second,
  ]
}

locals {
  first  = [for token in data.doit_service_account_tokens.all.items : token if token.id == doit_service_account_token.first.id]
  second = [for token in data.doit_service_account_tokens.all.items : token if token.id == doit_service_account_token.second.id]
}

output "token_count" {
  value = length(data.doit_service_account_tokens.all.items)
}

output "tokens_match" {
  value = (
    length(local.first) == 1 && length(local.second) == 1 &&
    local.first[0].name == doit_service_account_token.first.name &&
    local.first[0].state == "active" &&
    local.first[0].expires_time == "2099-01-01T00:00:00Z" &&
    local.first[0].service_account_id == doit_service_account.this.id &&
    local.first[0].customer_id == doit_service_account_token.first.customer_id &&
    local.first[0].create_time == doit_service_account_token.first.create_time &&
    local.second[0].name == doit_service_account_token.second.name &&
    local.second[0].state == "disabled" &&
    local.second[0].expires_time == doit_service_account_token.second.expires_time
  )
}
`, name)
}

func TestAccServiceAccountTokensDataSource_Empty(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-sats-ds-empty")
	config := fmt.Sprintf(`
resource "doit_service_account" "this" {
  name = %q
}

data "doit_service_account_tokens" "all" {
  service_account_id = doit_service_account.this.id
}
`, rName)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_service_account_tokens.all", tfjsonpath.New("items"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
