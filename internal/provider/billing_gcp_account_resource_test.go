package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccBillingGcpAccount_preCheck skips tests when the GCP billing account
// variables are not set. The account must not be connected to DoiT, and the
// DoiT service account from doit_billing_gcp_requirements must hold the
// required roles on it and on the export dataset.
func testAccBillingGcpAccount_preCheck(t *testing.T) {
	t.Helper()
	testAccPreCheckFunc(t)()
	for _, name := range []string{"TEST_GCP_BILLING_ACCOUNT_ID", "TEST_GCP_BILLING_EXPORT_PROJECT", "TEST_GCP_BILLING_EXPORT_DATASET"} {
		if os.Getenv(name) == "" {
			t.Skip(name + " not set")
		}
	}
}

func testAccBillingGcpAccountConfig() string {
	return fmt.Sprintf(`
resource "doit_billing_gcp_account" "this" {
  billing_account_id = %q
  export_project_id  = %q
  export_dataset_id  = %q
}
`, os.Getenv("TEST_GCP_BILLING_ACCOUNT_ID"), os.Getenv("TEST_GCP_BILLING_EXPORT_PROJECT"), os.Getenv("TEST_GCP_BILLING_EXPORT_DATASET"))
}

func TestAccBillingGcpAccount_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_GCP_BILLING_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 func() { testAccBillingGcpAccount_preCheck(t) },
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBillingGcpAccountConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_billing_gcp_account.this", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_billing_gcp_account.this", tfjsonpath.New("id"),
						knownvalue.StringExact(os.Getenv("TEST_GCP_BILLING_ACCOUNT_ID"))),
					statecheck.ExpectKnownValue("doit_billing_gcp_account.this", tfjsonpath.New("billing_account_id"),
						knownvalue.StringExact(os.Getenv("TEST_GCP_BILLING_ACCOUNT_ID"))),
					statecheck.ExpectKnownValue("doit_billing_gcp_account.this", tfjsonpath.New("export_project_id"),
						knownvalue.StringExact(os.Getenv("TEST_GCP_BILLING_EXPORT_PROJECT"))),
					statecheck.ExpectKnownValue("doit_billing_gcp_account.this", tfjsonpath.New("export_dataset_id"),
						knownvalue.StringExact(os.Getenv("TEST_GCP_BILLING_EXPORT_DATASET"))),
					statecheck.ExpectKnownValue("doit_billing_gcp_account.this", tfjsonpath.New("status"),
						knownvalue.NotNull()),
				},
			},
			{
				Config: testAccBillingGcpAccountConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccBillingGcpAccount_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: shares TEST_GCP_BILLING_ACCOUNT_ID
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 func() { testAccBillingGcpAccount_preCheck(t) },
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBillingGcpAccountConfig(),
			},
			{
				ResourceName:      "doit_billing_gcp_account.this",
				ImportState:       true,
				ImportStateVerify: true,
				// Activation runs in the background, so the status can move on
				// between create and import.
				ImportStateVerifyIgnore: []string{"status", "failure_reason", "timeouts"},
			},
			// Configuration generated for the import (an import block with
			// -generate-config-out) is valid and plans no change.
			{
				ResourceName:    "doit_billing_gcp_account.this",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
			{
				Config: testAccBillingGcpAccountConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
