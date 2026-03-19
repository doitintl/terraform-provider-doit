package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccCloudIncidentsDataSource_Basic tests basic cloud incidents data retrieval.
func TestAccCloudIncidentsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourceBasicConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_incidents.test", "incidents.#"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_incidents.test", "row_count"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccCloudIncidentsDataSourceBasicConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCloudIncidentsDataSourceBasicConfig() string {
	return `
data "doit_cloud_incidents" "test" {
  max_results = 10
}
`
}

// TestAccCloudIncidentsDataSource_SingleIncident tests fetching a specific cloud incident by ID.
func TestAccCloudIncidentsDataSource_SingleIncident(t *testing.T) {
	incidentID := os.Getenv("TEST_CLOUD_INCIDENT_ID")
	if incidentID == "" {
		t.Skip("TEST_CLOUD_INCIDENT_ID not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentSingleDataSourceConfig(incidentID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_incident.test", "id", incidentID),
					resource.TestCheckResourceAttrSet("data.doit_cloud_incident.test", "title"),
				),
			},
		},
	})
}

func testAccCloudIncidentSingleDataSourceConfig(incidentID string) string {
	return fmt.Sprintf(`
data "doit_cloud_incident" "test" {
  id = "%s"
}
`, incidentID)
}

// TestAccCloudIncidentsDataSource_Pagination tests manual pagination with page_token.
// Uses chained data sources to avoid page token expiry.
func TestAccCloudIncidentsDataSource_Pagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourceChainedPagination(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_incidents.second_page", "incidents.#"),
				),
			},
		},
	})
}

func testAccCloudIncidentsDataSourceChainedPagination() string {
	return `
data "doit_cloud_incidents" "first_page" {
  max_results = 1
}
data "doit_cloud_incidents" "second_page" {
  max_results = 5
  page_token  = data.doit_cloud_incidents.first_page.page_token
}
`
}
