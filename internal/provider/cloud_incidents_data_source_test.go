package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccCloudIncidentsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccCloudIncidentsDataSource_MaxResultsOnly(t *testing.T) {
	incidentCount := getCloudIncidentCount(t)
	if incidentCount < 3 {
		t.Skipf("Need at least 3 cloud incidents to test pagination, got %d", incidentCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourceMaxResultsConfig(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_incidents.limited", "incidents.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_incidents.limited", "page_token"),
				),
			},
			{
				Config:   testAccCloudIncidentsDataSourceMaxResultsConfig(2),
				PlanOnly: true,
			},
		},
	})
}

func testAccCloudIncidentsDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_cloud_incidents" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccCloudIncidentsDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccCloudIncidentsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getCloudIncidentFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 cloud incident)")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_cloud_incidents.from_token", "incidents.#"),
				),
			},
		},
	})
}

func testAccCloudIncidentsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_cloud_incidents" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccCloudIncidentsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccCloudIncidentsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getCloudIncidentFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 cloud incident)")
	}

	incidentCount := getCloudIncidentCount(t)
	if incidentCount < 3 {
		t.Skipf("Need at least 3 cloud incidents to test pagination, got %d", incidentCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourceMaxResultsAndPageTokenConfig(1, pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_incidents.paginated", "incidents.#", "1"),
				),
			},
		},
	})
}

func testAccCloudIncidentsDataSourceMaxResultsAndPageTokenConfig(maxResults int64, pageToken string) string {
	return fmt.Sprintf(`
data "doit_cloud_incidents" "paginated" {
  max_results = %d
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccCloudIncidentsDataSource_AutoPagination tests that without max_results, all incidents are fetched.
func TestAccCloudIncidentsDataSource_AutoPagination(t *testing.T) {
	expectedCount := getCloudIncidentCount(t)
	if expectedCount == 0 {
		t.Skip("No cloud incidents available to test auto-pagination")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudIncidentsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_incidents.test", "row_count", fmt.Sprintf("%d", expectedCount)),
					resource.TestCheckNoResourceAttr("data.doit_cloud_incidents.test", "page_token"),
				),
			},
		},
	})
}

func testAccCloudIncidentsDataSourceConfig() string {
	return `
data "doit_cloud_incidents" "test" {
}
`
}

// Helper functions

func getCloudIncidentCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListKnownIssuesParams{}
	for {
		resp, err := client.ListKnownIssuesWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list cloud incidents: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Incidents == nil {
			break
		}
		total += len(*resp.JSON200.Incidents)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}

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
