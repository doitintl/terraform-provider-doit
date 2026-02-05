package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"math/rand/v2"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccReport(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReport(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			{
				Config: testAccReportUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("test report updated")),
				},
			},
		},
	})
}

func TestAccReport_Minimal(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportMinimal(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-minimal-%d", n))),
				},
			},
		},
	})
}

func TestAccReport_Import(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportMinimal(n),
			},
			{
				ResourceName:      "doit_report.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccReport_Attributions(t *testing.T) {
	// Skip if not running acceptance tests - this check must come before
	// getValidAttributionAndGroup() which makes API calls
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	t.Parallel()

	// Dynamically fetch valid IDs
	attrID, groupID := getValidAttributionAndGroup(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 testAccPreCheckFunc(t),
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccReportAttributions(attrID, groupID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("doit_report.this", "config.filters.#", "1"),
					resource.TestCheckResourceAttr("doit_report.this", "config.filters.0.id", "attribution"),
					// Check logic ensures values contains the ID
					resource.TestCheckResourceAttr("doit_report.this", "config.filters.0.values.0", attrID),
					resource.TestCheckResourceAttr("doit_report.this", "config.group.#", "2"),
					resource.TestCheckResourceAttr("doit_report.this", "config.group.0.type", "attribution_group"),
					resource.TestCheckResourceAttr("doit_report.this", "config.group.0.id", groupID),
				),
			},
		},
	})
}

func getValidAttributionAndGroup(t *testing.T) (string, string) {
	token := os.Getenv("DOIT_API_TOKEN")
	host := os.Getenv("DOIT_HOST")
	customerContext := os.Getenv("DOIT_CUSTOMER_CONTEXT")

	if token == "" || host == "" {
		t.Skip("DOIT_API_TOKEN and DOIT_HOST must be set for dynamic ID fetching")
	}

	ctx := context.Background()

	client, err := provider.NewClient(ctx, host, token, customerContext)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// List Attributions
	attrRawResp, err := client.ListAttributions(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list attributions: %v", err)
	}
	defer func() {
		if closeErr := attrRawResp.Body.Close(); closeErr != nil {
			t.Logf("[WARN] Error closing response body: %v", closeErr)
		}
	}()

	var attrResult map[string]interface{}
	if decodeErr := json.NewDecoder(attrRawResp.Body).Decode(&attrResult); decodeErr != nil {
		t.Fatalf("Failed to decode attributions response: %v", decodeErr)
	}

	var attrID string
	if attrs, ok := attrResult["attributions"].([]interface{}); ok && len(attrs) > 0 {
		if aMap, ok := attrs[0].(map[string]interface{}); ok {
			if id, ok := aMap["id"].(string); ok {
				attrID = id
			}
		}
	}

	if attrID == "" {
		t.Skip("No valid attributions found to test with")
	}

	// List Attribution Groups
	groupRawResp, err := client.ListAttributionGroups(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list attribution groups: %v", err)
	}
	defer func() {
		if err := groupRawResp.Body.Close(); err != nil {
			t.Logf("[WARN] Error closing response body: %v", err)
		}
	}()

	var groupResult map[string]interface{}
	if err := json.NewDecoder(groupRawResp.Body).Decode(&groupResult); err != nil {
		t.Fatalf("Failed to decode attribution groups response: %v", err)
	}

	var groupID string
	foundGroupList := false
	for _, v := range groupResult {
		if list, ok := v.([]interface{}); ok {
			foundGroupList = true
			if len(list) > 0 {
				if gMap, ok := list[0].(map[string]interface{}); ok {
					if id, ok := gMap["id"].(string); ok {
						groupID = id
					}
				}
			}
			break
		}
	}

	if !foundGroupList || groupID == "" {
		t.Skip("No valid attribution groups found to test with")
	}

	return attrID, groupID
}

func testAccReportAttributions(attrID, groupID string) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
  name        = "test_report_attributions"
  description = "test_report_attributions"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation   = "total"
    time_interval = "month"
    filters = [
      {
        inverse = false
        id      = "attribution"
        type    = "attribution"
        values = [
          "%s"
        ]
        mode = "is"
      }
    ]
    group = [
      {
        id   = "%s"
        type = "attribution_group"
      },
      {
        id   = "cloud_provider"
        type = "fixed"
      }
    ]
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}
`, attrID, groupID)
}

func testAccReportMinimal(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-minimal-%d"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation    = "total"
		time_interval  = "month"
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

func TestAccReport_Full(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportFull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.this",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-full-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("custom_time_range").AtMapKey("from"),
						knownvalue.StringExact("2024-01-01T00:00:00Z")),
				},
			},
		},
	})
}

func testAccReportFull(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-full-%d"
	description = "Full report with splits and custom time"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		custom_time_range = {
		  from = "2024-01-01T00:00:00Z"
		  to   = "2024-02-01T00:00:00Z"
		}
		time_range = {
			mode = "custom"
			unit = "day"
		}
		advanced_analysis = {
		  trending_up   = true
		  trending_down = true
		  not_trending  = true
		  forecast      = true
		}

		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

func testAccReport(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-%d"
	description = "test report"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		metric_filter = {
			metric = {
				type = "basic"
				value = "cost"
			}
			operator = "nb"
			values = [-0.01, 0.01]
		}
		include_promotional_credits = false
		include_subtotals = false
		advanced_analysis = {
		  trending_up   = false
		  trending_down = false
		  not_trending  = false
		  forecast      = false
		}
		aggregation   = "total"
		time_interval = "month"
		dimensions = [
		  {
			id   = "year"
			type = "datetime"
		  },
		  {
			id   = "month"
			type = "datetime"
		  }
		]
		time_range = {
		  mode            = "last"
		  amount          = 6
		  include_current = true
		  unit            = "month"
		}
		filters = [
		  {
			id      = "cloud_provider"
			type    = "fixed"
			inverse = false
			values = [
			  "google-cloud"
			]
			mode = "is"
		  },
		  {
			id = "service_description"
			type = "fixed"
			inverse = false
			values = ["Kubernetes Engine"]
			mode = "is"
		  }
		]
		group = [
		  {
			id   = "sku_description"
			type = "fixed"
			limit = {
				value = 15
				sort = "desc"
				metric = {
					type = "basic"
					value = "cost"
				}
			}
		  },
		  {
			id   = "region"
			type = "fixed"
		  }
		]
		layout         = "stacked_column_chart"
		display_values = "actuals_only"
		currency       = "USD"
		sort_groups    = "asc"
		sort_dimensions = "a_to_z"
		data_source    = "billing"
	  }
}

`, i)
}

func testAccReportUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-%d"
	description = "test report updated"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		metric_filter = {
			metric = {
				type = "basic"
				value = "cost"
			}
			operator = "nb"
			values = [-0.01, 0.01]
		}
		include_promotional_credits = false
		include_subtotals = false
		advanced_analysis = {
		  trending_up   = false
		  trending_down = false
		  not_trending  = false
		  forecast      = false
		}
		aggregation   = "total"
		time_interval = "month"
		dimensions = [
		  {
			id   = "year"
			type = "datetime"
		  },
		  {
			id   = "month"
			type = "datetime"
		  }
		]
		time_range = {
		  mode            = "last"
		  amount          = 12
		  include_current = true
		  unit            = "month"
		}
		filters = [
		  {
			id      = "cloud_provider"
			type    = "fixed"
			inverse = false
			values = [
			  "google-cloud"
			]
			mode = "is"
		  },
		  {
			id = "service_description"
			type = "fixed"
			inverse = false
			values = ["Kubernetes Engine"]
			mode = "is"
		  }
		]
		group = [
		  {
			id   = "sku_description"
			type = "fixed"
			limit = {
				value = 15
				sort = "desc"
				metric = {
					type = "basic"
					value = "cost"
				}
			}
		  },
		  {
			id   = "region"
			type = "fixed"
		  }
		]
		layout         = "stacked_column_chart"
		display_values = "actuals_only"
		currency       = "USD"
		sort_groups    = "asc"
		sort_dimensions = "a_to_z"
		data_source    = "billing"
	  }
}
`, i)
}

// TestAccReport_CustomTimeRangeTimezonePreservation tests that custom_time_range timestamps
// with non-UTC timezone offsets are preserved correctly, avoiding "Provider produced inconsistent result" errors.
func TestAccReport_CustomTimeRangeTimezonePreservation(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithTimezoneOffset(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.timezone_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.timezone_test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-timezone-%d", n))),
					// Verify the timestamp is preserved with the original timezone offset
					statecheck.ExpectKnownValue(
						"doit_report.timezone_test",
						tfjsonpath.New("config").AtMapKey("custom_time_range").AtMapKey("from"),
						knownvalue.StringExact("2024-01-01T00:00:00-05:00")),
					statecheck.ExpectKnownValue(
						"doit_report.timezone_test",
						tfjsonpath.New("config").AtMapKey("custom_time_range").AtMapKey("to"),
						knownvalue.StringExact("2024-02-01T00:00:00-05:00")),
				},
			},
		},
	})
}

func testAccReportWithTimezoneOffset(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "timezone_test" {
    name = "test-timezone-%d"
	description = "Report with non-UTC timezone to test timestamp preservation"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		custom_time_range = {
		  # Use EST timezone offset (-05:00) instead of UTC
		  from = "2024-01-01T00:00:00-05:00"
		  to   = "2024-02-01T00:00:00-05:00"
		}
		time_range = {
			mode = "custom"
			unit = "day"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccReport_Disappears(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	var resourceId string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create the resource and capture ID
			{
				Config: testAccReportMinimal(n),
				Check: resource.ComposeTestCheckFunc(
					// Capture the resource ID for later deletion
					resource.TestCheckResourceAttrWith("doit_report.this", "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("resource ID is empty")
						}
						resourceId = value
						return nil
					}),
				),
			},
			// Step 2: Delete the resource via API, then verify Terraform detects the drift
			{
				PreConfig: func() {
					client := getAPIClient(t)
					resp, err := client.DeleteReportWithResponse(context.Background(), resourceId)
					if err != nil {
						t.Fatalf("Failed to delete report via API: %v", err)
					}
					if resp.StatusCode() != 200 && resp.StatusCode() != 204 && resp.StatusCode() != 404 {
						t.Fatalf("Expected 200, 204, or 404 from API, got %d: %s", resp.StatusCode(), string(resp.Body))
					}
				},
				Config:             testAccReportMinimal(n),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // Should detect deletion and plan to recreate
			},
		},
	})
}
