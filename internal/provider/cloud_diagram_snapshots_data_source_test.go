package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudDiagramSnapshotsDataSource_Basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless TF_ACC is set")
	}

	client := getAPIClient(t)
	layerID := findValidLayerID(client)
	if layerID == "" {
		t.Skip("Skipping because no valid cloud diagram layer ID was found in the environment")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudDiagramSnapshotsDataSourceConfig(layerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "id", layerID),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_snapshots.test", "cloud_diagram_snapshots.#"),
				),
			},
			{
				Config: testAccCloudDiagramSnapshotsDataSourceConfigWithOptions(layerID, 2, 0, "-createdAt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "id", layerID),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "limit", "2"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "offset", "0"),
					resource.TestCheckResourceAttr("data.doit_cloud_diagram_snapshots.test", "sort", "-createdAt"),
					resource.TestCheckResourceAttrSet("data.doit_cloud_diagram_snapshots.test", "cloud_diagram_snapshots.#"),
				),
			},
		},
	})
}

func TestAccCloudDiagramSnapshotsDataSource_NotFound(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless TF_ACC is set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudDiagramSnapshotsDataSourceConfig("nonexistent-layer-id-12345"),
				ExpectError: regexp.MustCompile(`(?i)not found|404|Cloud Diagram layer`),
			},
		},
	})
}

func findValidLayerID(client *models.ClientWithResponses) string {
	ctx := context.Background()

	// 1. Try to extract project ID from TEST_CLOUD_DIAGRAM_RESOURCE
	resourceURI := os.Getenv("TEST_CLOUD_DIAGRAM_RESOURCE")
	var queries []string
	if resourceURI != "" {
		parts := strings.Split(resourceURI, "/")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" && part != "projects" && part != "global" && part != "networks" && !strings.Contains(part, ".") {
				queries = append(queries, part)
			}
		}
	}
	// Add some other general queries to try
	queries = append(queries, "hannes-playground-411714", "peer-a", "hannes", "playground", "test", "a", "e", "i")

	for _, query := range queries {
		searchResp, err := client.SearchCloudDiagramsWithResponse(ctx, models.SearchCloudDiagramsJSONRequestBody{
			Query: query,
		})
		if err != nil {
			continue
		}
		if searchResp.StatusCode() == 200 && searchResp.JSON200 != nil && searchResp.JSON200.Scheme != nil {
			for _, scheme := range *searchResp.JSON200.Scheme {
				if scheme.UnderscoreId != "" {
					return scheme.UnderscoreId
				}
				if scheme.SsId != nil && *scheme.SsId != "" {
					return *scheme.SsId
				}
			}
		}
	}

	return ""
}

func testAccCloudDiagramSnapshotsDataSourceConfig(layerID string) string {
	return fmt.Sprintf(`
data "doit_cloud_diagram_snapshots" "test" {
  id = %q
}
`, layerID)
}

func testAccCloudDiagramSnapshotsDataSourceConfigWithOptions(layerID string, limit, offset int, sort string) string {
	return fmt.Sprintf(`
data "doit_cloud_diagram_snapshots" "test" {
  id     = %q
  limit  = %d
  offset = %d
  sort   = %q
}
`, layerID, limit, offset, sort)
}
