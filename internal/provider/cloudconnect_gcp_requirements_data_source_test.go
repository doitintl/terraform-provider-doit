package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccCloudconnectGcpRequirementsDataSource_Project(t *testing.T) {
	projectID := os.Getenv("TEST_GCP_PROJECT_ID")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck: func() {
			testAccPreCheckFunc(t)()
			if projectID == "" {
				t.Skip("TEST_GCP_PROJECT_ID not set")
			}
		},
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "doit_cloudconnect_gcp_requirements" "this" {
  scope_kind  = "gcp_project"
  project_id  = %q
  feature_ids = ["core"]
}
`, projectID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_cloudconnect_gcp_requirements.this", tfjsonpath.New("catalog_version"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_cloudconnect_gcp_requirements.this", tfjsonpath.New("features"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_cloudconnect_gcp_requirements.this", tfjsonpath.New("artifacts"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}
