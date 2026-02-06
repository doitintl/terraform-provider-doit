package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"math/rand/v2"

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
			// Verify no drift on re-apply
			{
				Config: testAccReportMinimal(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
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
	t.Parallel()

	attrID := os.Getenv("TEST_ATTRIBUTION")
	groupID := os.Getenv("TEST_ATTRIBUTION_GROUP")
	if attrID == "" || groupID == "" {
		t.Skip("TEST_ATTRIBUTION and TEST_ATTRIBUTION_GROUP must be set for this test")
	}

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
			// Verify no drift on re-apply
			{
				Config: testAccReportAttributions(attrID, groupID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
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
			// Verify no drift on re-apply
			{
				Config: testAccReportFull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
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

// TestAccReport_WithTargets tests reports with targets configuration.
// Note: targets is a write-only field and not returned in state.
// This causes drift on subsequent plans - see CMP-38157.
func TestAccReport_WithTargets(t *testing.T) {
	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	attrID := os.Getenv("TEST_ATTRIBUTION")
	if attrID == "" {
		t.Skip("TEST_ATTRIBUTION must be set for this test")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithTargets(n, attrID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_report.targets", "id"),
					resource.TestCheckResourceAttr("doit_report.targets", "config.layout", "table"),
				),
			},
			// TODO(CMP-38157): Enable drift verification once API returns targets field.
			// Currently skipped because targets is write-only and causes perpetual drift.
			// {
			// 	Config: testAccReportWithTargets(n, attrID),
			// 	ConfigPlanChecks: resource.ConfigPlanChecks{
			// 		PreApply: []plancheck.PlanCheck{
			// 			plancheck.ExpectEmptyPlan(),
			// 		},
			// 	},
			// },
		},
	})
}

func testAccReportWithTargets(i int, attrID string) string {
	return fmt.Sprintf(`
resource "doit_report" "targets" {
    name = "test-targets-%d"
    description = "Report with targets configuration"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        targets = [
            {
                type = "attribution"
                id   = "%s"
            }
        ]
    }
}
`, i, attrID)
}

// TestAccReport_WithSplits tests reports with splits configuration.
// Splits allow redistributing costs from one attribution to multiple targets.
//
// SKIP REASON: The splits API requires complex setup:
// - The origin attribution must be a member of the specified attribution group
// - The attribution group must have attributions with cost data for the split to work
// - TEST_ATTRIBUTION and TEST_ATTRIBUTION_GROUP must point to related entities
//
// To run this test manually:
// 1. Create an attribution group with at least 2 attributions
// 2. Set TEST_ATTRIBUTION_GROUP to the group ID
// 3. Set TEST_ATTRIBUTION to an attribution ID that is a member of that group.
func TestAccReport_WithSplits(t *testing.T) {
	// Skip: API bug - GET /reports/{id}/config returns 500 for reports with splits
	// See: https://doitintl.atlassian.net/browse/CMP-38160
	t.Skip("Skipped: API returns 500 when fetching config for reports with splits (CMP-38160)")

	n := rand.Int() //nolint:gosec // Weak random is fine for test data
	attrID := os.Getenv("TEST_ATTRIBUTION")
	attrGroupID := os.Getenv("TEST_ATTRIBUTION_GROUP")
	if attrID == "" || attrGroupID == "" {
		t.Skip("TEST_ATTRIBUTION and TEST_ATTRIBUTION_GROUP must be set for this test")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithSplits(n, attrID, attrGroupID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("doit_report.splits", "id"),
					resource.TestCheckResourceAttr("doit_report.splits", "config.layout", "table"),
				),
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithSplits(n, attrID, attrGroupID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSplits(i int, _ string, attrGroupID string) string {
	return fmt.Sprintf(`
resource "doit_report" "splits" {
    name = "test-splits-%d"
    description = "Report with splits configuration"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        splits = [
            {
                id   = "%s"
                type = "attribution_group"
                mode = "even"
                include_origin = false
                origin = {
                    id   = "unallocated"
                    type = "unallocated"
                }
                targets = []
            }
        ]
    }
}
`, i, attrGroupID)
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
