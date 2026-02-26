package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccReport(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
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
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
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
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
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
	attrID := os.Getenv("TEST_ATTRIBUTION")
	groupID := os.Getenv("TEST_ATTRIBUTION_GROUP")
	if attrID == "" || groupID == "" {
		t.Skip("TEST_ATTRIBUTION and TEST_ATTRIBUTION_GROUP must be set for this test")
	}

	resource.ParallelTest(t, resource.TestCase{
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
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
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
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
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

// TestAccReport_WithSplits tests reports with splits configuration.
// Splits allow redistributing costs from one attribution to multiple targets.
//
// To run this test:
// 1. Set TEST_ATTRIBUTION_GROUP to an attribution group ID
// 2. Set TEST_ATTRIBUTION to an attribution ID that is a member of that group.
func TestAccReport_WithSplits(t *testing.T) {
	n := acctest.RandInt()
	attrID := os.Getenv("TEST_ATTRIBUTION")
	attrGroupID := os.Getenv("TEST_ATTRIBUTION_GROUP")
	if attrID == "" || attrGroupID == "" {
		t.Skip("TEST_ATTRIBUTION and TEST_ATTRIBUTION_GROUP must be set for this test")
	}

	resource.ParallelTest(t, resource.TestCase{
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

func testAccReportWithSplits(i int, attrID, attrGroupID string) string {
	return fmt.Sprintf(`
resource "doit_report" "splits" {
    name = "test-splits-%d"
    description = "Report with splits configuration"
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
        # group must include the attribution group used in splits
        group = [
            {
                id   = "%s"
                type = "attribution_group"
            }
        ]
        splits = [
            {
                id   = "%s"
                type = "attribution_group"
                mode = "even"
                include_origin = true
                origin = {
                    id   = "%s"
                    type = "attribution"
                }
                targets = []
            }
        ]
    }
}
`, i, attrGroupID, attrGroupID, attrID)
}

// TestAccReport_WithSplitTargets tests reports with populated splits[].targets.
// This verifies that explicit targets within a split are correctly sent to the API,
// returned in the config, and cause no drift on subsequent plans.
// Uses a fixed-dimension split (cloud_provider) with custom mode to avoid
// attribution-specific constraints around origin/target uniqueness.
func TestAccReport_WithSplitTargets(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithSplitTargets(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.split_targets",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.split_targets",
						tfjsonpath.New("config").AtMapKey("splits").AtSliceIndex(0).AtMapKey("targets"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type": knownvalue.StringExact("fixed"),
							}),
						})),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithSplitTargets(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSplitTargets(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "split_targets" {
    name = "test-split-targets-%d"
    description = "Report with populated split targets"
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
        group = [
            {
                id   = "cloud_provider"
                type = "fixed"
            }
        ]
        splits = [
            {
                id   = "cloud_provider"
                type = "fixed"
                mode = "custom"
                include_origin = true
                origin = {
                    id   = "google-cloud"
                    type = "fixed"
                }
                targets = [
                    {
                        id    = "amazon-web-services"
                        type  = "fixed"
                        value = 1.0
                    }
                ]
            }
        ]
    }
}
`, i)
}

// TestAccReport_Disappears verifies that Terraform correctly handles
// resources that are deleted outside of Terraform (externally deleted).
// This tests the Read method's 404 handling and RemoveResource call.
func TestAccReport_Disappears(t *testing.T) {
	n := acctest.RandInt()
	var resourceId string

	resource.ParallelTest(t, resource.TestCase{
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

// TestAccReport_WithEmptyLists tests that explicitly setting list fields to [] works correctly.
// Tests the provider fix: API returns null for empty lists, but provider normalizes to [].
func TestAccReport_WithEmptyLists(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithEmptyLists(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("dimensions"),
						knownvalue.ListExact([]knownvalue.Check{})), // Empty list
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{})), // Empty list
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("group"),
						knownvalue.ListExact([]knownvalue.Check{})), // Empty list
				},
			},
			// Step 2: Re-apply same config - verify no drift.
			// If toExternalConfig returns null instead of [] for these lists,
			// Terraform will see config ([]) ≠ state (null) and produce a non-empty plan.
			{
				Config: testAccReportWithEmptyLists(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithEmptyLists(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
  name = "test-empty-lists-%d"
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
    dimensions     = []
    filters        = []
    group          = []
  }
}
`, i)
}

// TestAccReport_WithFilterEmptyValues tests that a filter with values = [] is handled
// correctly. This exercises the report filter value handling logic where types.ListNull
// was used for filter values when the API returned nil.
func TestAccReport_WithFilterEmptyValues(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithFilterEmptyValues(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.filter_empty_values",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Step 2: Re-apply - verify no drift
			{
				Config: testAccReportWithFilterEmptyValues(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithFilterEmptyValues(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "filter_empty_values" {
  name = "test-filter-empty-values-%d"
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
    filters = [
      {
        id      = "cloud_provider"
        type    = "fixed"
        inverse = false
        values  = []
        mode    = "is"
      }
    ]
  }
}
`, i)
}

// TestAccReport_WithMetricFilterEmptyValues tests that a metric_filter with values = []
// is rejected by the API. Unlike filter.values (which accepts []), the API requires
// at least one value for metric_filter.
// The code fix in report.go (returning empty list instead of null) is still correct
// defensively for when the API returns nil on read, but users can't trigger the
// inconsistent result bug from HCL because the API blocks it.
func TestAccReport_WithMetricFilterEmptyValues(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportWithMetricFilterEmptyValues(n),
				ExpectError: regexp.MustCompile(`invalid number of values`),
			},
		},
	})
}

func testAccReportWithMetricFilterEmptyValues(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "metric_filter_empty" {
  name = "test-mf-empty-values-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    metric_filter = {
      metric = {
        type  = "basic"
        value = "cost"
      }
      operator = "nb"
      values   = []
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

// TestAccReport_WithMetrics tests reports using the new metrics list (replaces deprecated singular metric).
// This validates that multiple metrics (up to 4) work correctly.
func TestAccReport_WithMetrics(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithMetrics(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.metrics_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.metrics_test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-metrics-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_report.metrics_test",
						tfjsonpath.New("config").AtMapKey("metrics"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"type":  knownvalue.StringExact("basic"),
								"value": knownvalue.StringExact("cost"),
							}),
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"type":  knownvalue.StringExact("basic"),
								"value": knownvalue.StringExact("usage"),
							}),
						})),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithMetrics(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithMetrics(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "metrics_test" {
    name = "test-metrics-%d"
    description = "Report using new metrics list (multiple metrics)"
    config = {
        # Use new metrics list instead of deprecated singular metric
        metrics = [
            {
                type  = "basic"
                value = "cost"
            },
            {
                type  = "basic"
                value = "usage"
            }
        ]
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

// TestAccReport_WithoutDataSource verifies that the API accepts report creation
// without the data_source field. In v0.26.0 the field didn't exist in the provider
// at all (never sent to the API). The OpenAPI spec marks it as optional.
// This test confirms no regression when omitting data_source.
func TestAccReport_WithoutDataSource(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithoutDataSource(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.no_datasource",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.no_datasource",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-no-ds-%d", n))),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithoutDataSource(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithoutDataSource(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "no_datasource" {
    name = "test-no-ds-%d"
    description = "Report without data_source to verify it is truly optional"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation    = "total"
        time_interval  = "month"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_DataSource tests that non-default dataSource values
// (specifically billing-datahub) work correctly after fixing the enum
// mismatch between the OpenAPI spec and actual API.
// See: https://github.com/doitintl/terraform-provider-doit/issues/74
func TestAccReport_DataSource(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with billing-datahub data source
			{
				Config: testAccReportDataSource(n, "billing-datahub"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.datasource_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.datasource_test",
						tfjsonpath.New("config").AtMapKey("data_source"),
						knownvalue.StringExact("billing-datahub")),
				},
			},
			// Step 2: Update to bqlens data source
			{
				Config: testAccReportDataSource(n, "bqlens"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.datasource_test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.datasource_test",
						tfjsonpath.New("config").AtMapKey("data_source"),
						knownvalue.StringExact("bqlens")),
				},
			},
			// Step 3: Verify no drift on re-apply
			{
				Config: testAccReportDataSource(n, "bqlens"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportDataSource(i int, dataSource string) string {
	return fmt.Sprintf(`
resource "doit_report" "datasource_test" {
    name = "test-datasource-%d"
    description = "Report testing dataSource enum values"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "%s"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i, dataSource)
}

// TestAccReport_SecondaryTimeRange tests reports with a relative secondary time range
// (e.g., compare to last year). Validates create and drift-free re-apply.
func TestAccReport_SecondaryTimeRange(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithSecondaryTimeRange(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.secondary_tr",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.secondary_tr",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("amount"),
						knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(
						"doit_report.secondary_tr",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("unit"),
						knownvalue.StringExact("year")),
					statecheck.ExpectKnownValue(
						"doit_report.secondary_tr",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("include_current"),
						knownvalue.Bool(false)),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithSecondaryTimeRange(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSecondaryTimeRange(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "secondary_tr" {
    name = "test-sec-tr-%d"
    description = "Report with secondary time range (relative)"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        time_range = {
          mode            = "last"
          amount          = 3
          include_current = true
          unit            = "month"
        }
        secondary_time_range = {
          amount          = 1
          unit            = "year"
          include_current = false
        }
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_SecondaryTimeRangeCustom tests a secondary time range with
// an explicit custom date range, verifying timestamps are preserved in state.
func TestAccReport_SecondaryTimeRangeCustom(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithSecondaryTimeRangeCustom(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.secondary_custom",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.secondary_custom",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("from"),
						knownvalue.StringExact("2023-01-01T00:00:00Z")),
					statecheck.ExpectKnownValue(
						"doit_report.secondary_custom",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("to"),
						knownvalue.StringExact("2023-12-31T23:59:59Z")),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithSecondaryTimeRangeCustom(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSecondaryTimeRangeCustom(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "secondary_custom" {
    name = "test-sec-custom-%d"
    description = "Report with secondary time range using custom dates"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        custom_time_range = {
          from = "2024-01-01T00:00:00Z"
          to   = "2024-12-31T23:59:59Z"
        }
        time_range = {
          mode = "custom"
          unit = "day"
        }
        secondary_time_range = {
          custom_time_range = {
            from = "2023-01-01T00:00:00Z"
            to   = "2023-12-31T23:59:59Z"
          }
        }
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_SecondaryTimeRangeUpdate tests updating a report's secondary time range
// from a relative comparison (year-over-year) to a custom date range in a multi-step test.
func TestAccReport_SecondaryTimeRangeUpdate(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with relative secondary time range (year-over-year)
			{
				Config: testAccReportSecondaryTimeRangeStep1(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.secondary_update",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("amount"),
						knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(
						"doit_report.secondary_update",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("unit"),
						knownvalue.StringExact("year")),
				},
			},
			// Verify no drift
			{
				Config: testAccReportSecondaryTimeRangeStep1(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 2: Update to a custom date range secondary time range
			{
				Config: testAccReportSecondaryTimeRangeStep2(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.secondary_update",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.secondary_update",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("from"),
						knownvalue.StringExact("2023-01-01T00:00:00Z")),
					statecheck.ExpectKnownValue(
						"doit_report.secondary_update",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("to"),
						knownvalue.StringExact("2023-12-31T23:59:59Z")),
				},
			},
			// Verify no drift after update
			{
				Config: testAccReportSecondaryTimeRangeStep2(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportSecondaryTimeRangeStep1(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "secondary_update" {
    name = "test-sec-update-%d"
    description = "Report testing secondary time range updates"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        time_range = {
          mode            = "last"
          amount          = 3
          include_current = true
          unit            = "month"
        }
        secondary_time_range = {
          amount          = 1
          unit            = "year"
          include_current = false
        }
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

func testAccReportSecondaryTimeRangeStep2(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "secondary_update" {
    name = "test-sec-update-%d"
    description = "Report testing secondary time range updates"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        custom_time_range = {
          from = "2024-01-01T00:00:00Z"
          to   = "2024-12-31T23:59:59Z"
        }
        time_range = {
          mode = "custom"
          unit = "day"
        }
        secondary_time_range = {
          custom_time_range = {
            from = "2023-01-01T00:00:00Z"
            to   = "2023-12-31T23:59:59Z"
          }
        }
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_InvalidTimestamp verifies that invalid RFC3339 timestamps in
// custom_time_range.from are caught at plan time by the reportTimestampValidator,
// rather than waiting for API rejection at apply time.
func TestAccReport_InvalidTimestamp(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportInvalidTimestamp(n),
				ExpectError: regexp.MustCompile(`Invalid RFC3339 Timestamp`),
			},
		},
	})
}

func testAccReportInvalidTimestamp(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "invalid_ts" {
  name = "test-invalid-ts-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation   = "total"
    time_interval = "month"
    custom_time_range = {
      from = "not-a-valid-timestamp"
      to   = "2024-12-31T23:59:59Z"
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

// TestAccReport_InvalidSecondaryTimestamp verifies that invalid RFC3339 timestamps
// in secondary_time_range.custom_time_range.to are also caught at plan time.
func TestAccReport_InvalidSecondaryTimestamp(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportInvalidSecondaryTimestamp(n),
				ExpectError: regexp.MustCompile(`Invalid RFC3339 Timestamp`),
			},
		},
	})
}

func testAccReportInvalidSecondaryTimestamp(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "invalid_sec_ts" {
  name = "test-invalid-sec-ts-%d"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation   = "total"
    time_interval = "month"
    custom_time_range = {
      from = "2024-01-01T00:00:00Z"
      to   = "2024-12-31T23:59:59Z"
    }
    time_range = {
      mode = "custom"
      unit = "day"
    }
    secondary_time_range = {
      custom_time_range = {
        from = "2023-01-01T00:00:00Z"
        to   = "2023-31-12"
      }
    }
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}
`, i)
}
