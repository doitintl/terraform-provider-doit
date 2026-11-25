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

const testAccGcpConnection = "doit_cloudconnect_gcp_connection.this"

// testAccCloudconnectGcpConnection_preCheck skips tests when the GCP Cloud
// Connect variables are not set. The service account must hold the roles that
// doit_cloudconnect_gcp_requirements returns on the project, and must not be
// registered in another connection.
func testAccCloudconnectGcpConnection_preCheck(t *testing.T) {
	t.Helper()
	testAccPreCheckFunc(t)()
	for _, name := range []string{"TEST_GCP_PROJECT_ID", "TEST_GCP_SERVICE_ACCOUNT_EMAIL", "TEST_GCP_SERVICE_ACCOUNT_UNIQUE_ID"} {
		if os.Getenv(name) == "" {
			t.Skip(name + " not set")
		}
	}
}

// testAccCloudconnectGcpConnectionConfig declares a project connection. extra
// is added to the resource block as is.
func testAccCloudconnectGcpConnectionConfig(featureIDs, extra string) string {
	projectID := os.Getenv("TEST_GCP_PROJECT_ID")
	return fmt.Sprintf(`
resource "doit_cloudconnect_gcp_connection" "this" {
  scope_kind  = "gcp_project"
  project_id  = %[1]q
  feature_ids = %[4]s

  service_account = {
    email      = %[2]q
    unique_id  = %[3]q
    project_id = %[1]q
  }
%[5]s
}
`, projectID, os.Getenv("TEST_GCP_SERVICE_ACCOUNT_EMAIL"), os.Getenv("TEST_GCP_SERVICE_ACCOUNT_UNIQUE_ID"), featureIDs, extra)
}

func testAccGcpFeatureInputs(location string) string {
	return fmt.Sprintf(`
  feature_inputs = {
    bigquery_finops = {
      location = %q
    }
  }
`, location)
}

func TestAccCloudconnectGcpConnection_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: one connection per service account
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 func() { testAccCloudconnectGcpConnection_preCheck(t) },
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccGcpConnection, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("scope_kind"), knownvalue.StringExact("gcp_project")),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("project_id"),
						knownvalue.StringExact(os.Getenv("TEST_GCP_PROJECT_ID"))),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("organization_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("dataset_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_ids"),
						knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("core")})),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_inputs"), knownvalue.Null()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("role_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("catalog_version"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("etag"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("registered_via"), knownvalue.StringExact("api")),
				},
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccCloudconnectGcpConnection_FeatureInputs sets the BigQuery FinOps
// inputs, changes them in place and clears them together with a change of
// feature_ids. The service account needs the BigQuery FinOps permissions.
func TestAccCloudconnectGcpConnection_FeatureInputs(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: one connection per service account
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccCloudconnectGcpConnection_preCheck(t)
			if os.Getenv("TEST_GCP_BIGQUERY_FINOPS") == "" {
				t.Skip("TEST_GCP_BIGQUERY_FINOPS not set")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, testAccGcpFeatureInputs("EU")),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_inputs").AtMapKey("bigquery_finops").AtMapKey("location"),
						knownvalue.StringExact("EU")),
				},
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, testAccGcpFeatureInputs("EU")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, testAccGcpFeatureInputs("US")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccGcpConnection, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_inputs").AtMapKey("bigquery_finops").AtMapKey("location"),
						knownvalue.StringExact("US")),
				},
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, testAccGcpFeatureInputs("US")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Removing the inputs in the same step as a feature change clears them.
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccGcpConnection, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_inputs"), knownvalue.Null()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_ids"),
						knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("core")})),
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_inputs"), knownvalue.Null()),
				},
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Removing the inputs alone clears them too.
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, testAccGcpFeatureInputs("EU")),
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccGcpConnection, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccGcpConnection, tfjsonpath.New("feature_inputs"), knownvalue.Null()),
				},
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core", "bigquery-finops"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCloudconnectGcpConnection_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{ //nolint:paralleltest // sequential: one connection per service account
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 func() { testAccCloudconnectGcpConnection_preCheck(t) },
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core"]`, ""),
			},
			{
				ResourceName:      testAccGcpConnection,
				ImportState:       true,
				ImportStateVerify: true,
				// Background checks and follow-up steps can move on between
				// create and import.
				ImportStateVerifyIgnore: []string{
					"etag", "revision", "baseline_checks", "feature_verification", "follow_ups", "timeouts",
				},
			},
			// Configuration generated for the import (an import block with
			// -generate-config-out) is valid and plans no change.
			{
				ResourceName:    testAccGcpConnection,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				GenerateConfig:  true,
			},
			{
				Config: testAccCloudconnectGcpConnectionConfig(`["core"]`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
