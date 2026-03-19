package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
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
func TestAccCloudIncidentsDataSource_Pagination(t *testing.T) {
	pageToken := getCloudIncidentFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 cloud incident)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourcePaginationConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_incidents.test", "incidents.#"),
				),
			},
		},
	})
}

func testAccCloudIncidentsDataSourcePaginationConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_cloud_incidents" "test" {
  max_results = 5
  page_token  = "%s"
}
`, pageToken)
}

// Helper functions

func getCloudIncidentFirstPageToken(t *testing.T, maxResults int64) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	resp, err := client.ListKnownIssuesWithResponse(ctx, &models.ListKnownIssuesParams{
		MaxResults: &maxResults,
	})
	if err != nil {
		t.Fatalf("Failed to list cloud incidents: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
