package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBillingGcpRequirementsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `data "doit_billing_gcp_requirements" "this" {}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_billing_gcp_requirements.this", tfjsonpath.New("service_account_email"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_billing_gcp_requirements.this", tfjsonpath.New("grants"),
						knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("data.doit_billing_gcp_requirements.this", tfjsonpath.New("prerequisites"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}
