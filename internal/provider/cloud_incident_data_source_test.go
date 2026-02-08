package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudIncidentDataSource_Basic(t *testing.T) {
	incidentID := os.Getenv("TEST_CLOUD_INCIDENT_ID")
	if incidentID == "" {
		t.Skip("TEST_CLOUD_INCIDENT_ID environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentDataSourceConfig(incidentID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_incident.test", "id", incidentID),
					resource.TestCheckResourceAttrSet("data.doit_cloud_incident.test", "title"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_incident.test", "platform"),
				),
			},
		},
	})
}

func testAccCloudIncidentDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "doit_cloud_incident" "test" {
  id = %[1]q
}
`, id)
}
