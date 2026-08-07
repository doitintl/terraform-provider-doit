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
			// Drift detection: re-apply same config, expect no changes.
			{
				Config: testAccReport(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
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

// TestAccReport_OmittedOptionalComputed verifies that omitting all Optional+Computed
// fields doesn't cause drift. This is the core plan-first validation: Unknown fields
// must resolve correctly on Create and match what Read returns on refresh.
func TestAccReport_OmittedOptionalComputed(t *testing.T) {
	n := acctest.RandInt()

	// Shared state checks for omitted list fields — they should resolve to empty lists
	// or API-defaulted lists, not null. The useNullForUnknownListWhenConfigNull modifier
	// plans [] (not null) when config is omitted, so the overlay and Read paths stay simple.
	// Note: dimensions and metrics are API-defaulted (the API populates defaults when
	// omitted), so we don't assert their exact size — just that they exist (not null).
	listSizeChecks := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(
			"doit_report.this",
			tfjsonpath.New("labels"),
			knownvalue.ListSizeExact(0)),
		statecheck.ExpectKnownValue(
			"doit_report.this",
			tfjsonpath.New("config").AtMapKey("filters"),
			knownvalue.ListSizeExact(0)),
		statecheck.ExpectKnownValue(
			"doit_report.this",
			tfjsonpath.New("config").AtMapKey("group"),
			knownvalue.ListSizeExact(0)),
		statecheck.ExpectKnownValue(
			"doit_report.this",
			tfjsonpath.New("config").AtMapKey("splits"),
			knownvalue.ListSizeExact(0)),
		// dimensions and metrics are API-defaulted — assert they are known (not null)
		// without asserting exact size since the API populates defaults when omitted.
		statecheck.ExpectKnownValue(
			"doit_report.this",
			tfjsonpath.New("config").AtMapKey("dimensions"),
			knownvalue.NotNull()),
		statecheck.ExpectKnownValue(
			"doit_report.this",
			tfjsonpath.New("config").AtMapKey("metrics"),
			knownvalue.NotNull()),
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:            testAccReportMinimal(n),
				ConfigStateChecks: listSizeChecks,
			},
			// Verify no drift on re-apply
			{
				Config:            testAccReportMinimal(n),
				ConfigStateChecks: listSizeChecks,
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
				// config.metric and config.metrics are two views of one API field.
				// Import has no config to tell which one is in use, so it populates the
				// canonical metrics and leaves the deprecated metric mirror null (see
				// mapReportToModel). Trailing dots keep config.metric_filter.* matched.
				ImportStateVerifyIgnore: []string{"config.metric.", "config.metrics."},
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

// TestAccReport_AliasTypes verifies that using the new alias type names
// ("allocation" for "attribution_group", "allocation_rule" for "attribution")
// round-trips correctly without causing drift. The API returns canonical names
// but our normalizer preserves the user's alias choice in state.
func TestAccReport_AliasTypes(t *testing.T) {
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
				Config: testAccReportAliasTypes(attrID, groupID),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify alias types and ids are preserved in state
					resource.TestCheckResourceAttr("doit_report.alias_test", "config.filters.0.type", "allocation_rule"),
					resource.TestCheckResourceAttr("doit_report.alias_test", "config.filters.0.id", "allocation_rule"),
					resource.TestCheckResourceAttr("doit_report.alias_test", "config.group.0.type", "allocation"),
					resource.TestCheckResourceAttr("doit_report.alias_test", "config.group.0.id", groupID),
				),
			},
			// Verify no drift on re-apply - this is the critical test:
			// API returns "attribution"/"attribution_group" but normalizer
			// preserves user's "allocation_rule"/"allocation".
			{
				Config: testAccReportAliasTypes(attrID, groupID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportAliasTypes(attrID, groupID string) string {
	return fmt.Sprintf(`
resource "doit_report" "alias_test" {
  name        = "test_report_alias_types"
  description = "Test report using alias dimension types"
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
        id      = "allocation_rule"
        type    = "allocation_rule"
        values = [
          "%s"
        ]
        mode = "is"
      }
    ]
    group = [
      {
        id   = "%s"
        type = "allocation"
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
			// Re-apply must produce no plan: the -05:00 offset survives the Read-path
			// comparison (time.Time.Equal) for primary custom_time_range.
			{
				Config: testAccReportWithTimezoneOffset(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
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
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"id":    knownvalue.StringExact("amazon-web-services"),
								"type":  knownvalue.StringExact("fixed"),
								"value": knownvalue.Float64Exact(1.0),
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

// TestAccReport_SecondaryCustomTimeRangeTimezonePreservation round-trips
// secondary_time_range.custom_time_range with a non-UTC offset (-05:00) and
// drift-checks, verifying the offset is preserved with no drift.
func TestAccReport_SecondaryCustomTimeRangeTimezonePreservation(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithSecondaryTimeRangeOffset(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.secondary_offset",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("from"),
						knownvalue.StringExact("2023-01-01T00:00:00-05:00")),
					statecheck.ExpectKnownValue(
						"doit_report.secondary_offset",
						tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("to"),
						knownvalue.StringExact("2023-12-31T23:59:59-05:00")),
				},
			},
			// Drift check — exercises the secondary Read path (time.Time.Equal).
			{
				Config: testAccReportWithSecondaryTimeRangeOffset(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSecondaryTimeRangeOffset(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "secondary_offset" {
    name = "test-sec-offset-%d"
    description = "Report with secondary custom time range using non-UTC offset"
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
            from = "2023-01-01T00:00:00-05:00"
            to   = "2023-12-31T23:59:59-05:00"
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

// TestAccReport_WithOmittedLabels tests that omitting the labels attribute
// results in an empty list in state with no drift on re-apply.
func TestAccReport_WithOmittedLabels(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithOmittedLabels(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("labels"),
						knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithOmittedLabels(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithOmittedLabels(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
  name   = "test-omitted-labels-%d"
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

// TestAccReport_WithLabels tests that labels can be assigned to reports,
// verified uniquely, updated, and removed (set to []).
func TestAccReport_WithLabels(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create report with one label
			{
				Config: testAccReportWithLabel(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("labels"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Step 2: Verify no drift on re-apply
			{
				Config: testAccReportWithLabel(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Remove labels (set to [])
			{
				Config: testAccReportWithNoLabels(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("labels"),
						knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Step 4: Verify no drift after removing labels
			{
				Config: testAccReportWithNoLabels(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithLabel(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "test-report-label-%d"
  color = "blue"
}

resource "doit_report" "this" {
  name   = "test-labels-%d"
  labels = [doit_label.test.id]
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
`, i, i)
}

func testAccReportWithNoLabels(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "test" {
  name  = "test-report-label-%d"
  color = "blue"
}

resource "doit_report" "this" {
  name   = "test-labels-%d"
  labels = []
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
`, i, i)
}

// TestAccReport_IncludeNull tests that the include_null property on report filters
// round-trips correctly without causing drift.
func TestAccReport_IncludeNull(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithIncludeNull(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("country"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								"values":       knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("BE")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithIncludeNull(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithIncludeNull(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-include-null-%d"
	description = "Report testing include_null filter property"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		filters = [
		  {
			id           = "country"
			type         = "fixed"
			inverse      = false
			include_null = true
			values       = ["BE"]
			mode         = "is"
		  }
		]
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_FilterValuesPreservedOnUpdate verifies that filter values survive
// a report update that modifies an unrelated field (labels). This reproduces the
// scenario where the API's ToFilter() normalization dropped filter values during
// an update, causing "element 0 has vanished" errors.
// See: https://github.com/doitintl/df-dci-automations/actions/runs/23549764622
func TestAccReport_FilterValuesPreservedOnUpdate(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create a report with filters and no labels
			{
				Config: testAccReportFilterValuesStep1(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.filter_update",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.filter_update",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("service_description"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								"values":       knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("Compute Engine")}),
							}),
						}),
					),
				},
			},
			// Step 2: Verify no drift before update
			{
				Config: testAccReportFilterValuesStep1(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Update by adding a label — filter values must survive
			{
				Config: testAccReportFilterValuesStep2(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.filter_update",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify labels were added
					statecheck.ExpectKnownValue(
						"doit_report.filter_update",
						tfjsonpath.New("labels"),
						knownvalue.ListSizeExact(1)),
					// Verify filter values survived the update
					statecheck.ExpectKnownValue(
						"doit_report.filter_update",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("service_description"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								"values":       knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("Compute Engine")}),
							}),
						}),
					),
				},
			},
			// Step 4: Verify no drift after update
			{
				Config: testAccReportFilterValuesStep2(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccReport_CaseInsensitive tests that the case_insensitive property on report filters
// round-trips correctly without causing drift. Uses mode="contains" with a lowercase
// value to exercise the case-insensitive matching path.
func TestAccReport_CaseInsensitive(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithCaseInsensitive(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":             knownvalue.StringExact("fixed"),
								"id":               knownvalue.StringExact("country"),
								"mode":             knownvalue.StringExact("contains"),
								"case_insensitive": knownvalue.Bool(true),
								"values":           knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("be")}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportWithCaseInsensitive(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// Step 1: Report with filter, no labels.
func testAccReportFilterValuesStep1(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "filter_update" {
    name = "test-filter-update-%d"
    description = "Report testing filter values survive update"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id           = "service_description"
            type         = "fixed"
            include_null = true
            values       = ["Compute Engine"]
            mode         = "is"
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// Step 2: Same report but with a label added (triggers update).
func testAccReportFilterValuesStep2(i int) string {
	return fmt.Sprintf(`
resource "doit_label" "filter_test" {
  name  = "test-filter-update-label-%d"
  color = "blue"
}

resource "doit_report" "filter_update" {
    name = "test-filter-update-%d"
    description = "Report testing filter values survive update"
    labels = [doit_label.filter_test.id]
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id           = "service_description"
            type         = "fixed"
            include_null = true
            values       = ["Compute Engine"]
            mode         = "is"
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i, i)
}

func testAccReportWithCaseInsensitive(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-case-insensitive-%d"
	description = "Report testing case_insensitive filter property"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		filters = [
		  {
			id               = "country"
			type             = "fixed"
			inverse          = false
			case_insensitive = true
			values           = ["be"]
			mode             = "contains"
		  }
		]
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_FilterWithoutInverse verifies that a filter config omitting
// the optional `inverse` attribute creates successfully and produces no drift
// on re-apply. The `inverse` field uses ValueBoolPointer() so it is sent as
// nil (omitted via omitempty) when unset, matching alert/budget behaviour.
func TestAccReport_FilterWithoutInverse(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportFilterWithoutInverse(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.no_inverse",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.no_inverse",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("cloud_provider"),
								"mode":   knownvalue.StringExact("is"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("google-cloud")}),
							}),
						}),
					),
				},
			},
			// Re-apply the same config and assert an empty plan,
			// confirming the omitted inverse field causes no state drift.
			{
				Config: testAccReportFilterWithoutInverse(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportFilterWithoutInverse(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "no_inverse" {
    name = "test-no-inverse-%d"
	description = "Report testing filter without inverse field set"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		filters = [
		  {
			id     = "cloud_provider"
			type   = "fixed"
			values = ["google-cloud"]
			mode   = "is"
		  }
		]
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_FilterValuesNAStripped reproduces the exact bug where using legacy
// "[... N/A]" filter values causes "Provider produced inconsistent result" errors.
// The DoIT API silently strips these values and converts them to includeNull: true.
// For example, sending values=["[Customer N/A]"] returns values=[] + includeNull=true.
// Without the defensive fix, the provider sees the values vanish and crashes.
// See: https://github.com/doitintl/df-dci-automations/actions/runs/23655331590
// See: https://doitintl.atlassian.net/browse/CMP-38116
func TestAccReport_FilterValuesNAStripped(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with [Customer N/A] — the API will strip this value
			// and return values=[] + includeNull=true. The provider must preserve
			// the user's configured values to avoid "element 0 has vanished".
			{
				Config: testAccReportFilterNAStrippedConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.filter_na_stripped",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"values": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("[Customer N/A]"),
								}),
							}),
						}),
					),
				},
			},
			// Step 2: Verify no drift on re-plan — this is the critical check.
			// Without the fix, the provider would return values=[] from the API
			// response, causing Terraform to detect drift every time.
			{
				Config: testAccReportFilterNAStrippedConfig(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccReport_FilterValuesMixedWithNA covers the scenario where a filter
// contains BOTH a real value AND a legacy "[... N/A]" sentinel, e.g.:
//
//	values = ["Compute Engine", "[Service N/A]"]
//
// The API strips the sentinel and returns values=["Compute Engine"] + includeNull=true.
// Unlike the pure-NA case (where the API returns values=[] and the blunt fallback fires),
// here apiHasValues=true, and the provider must use include_null-based detection to
// restore the "[Service N/A]" sentinel so that state continues to match configuration
// and no perpetual drift is introduced.
// See: https://doitintl.atlassian.net/browse/CMP-38116
func TestAccReport_FilterValuesMixedWithNA(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with ["Compute Engine", "[Service N/A]"].
			// The API will return values=["Compute Engine"] + includeNull=true.
			// The provider must restore the "[Service N/A]" sentinel so that
			// the state matches the plan exactly.
			{
				Config: testAccReportFilterMixedNAConfig(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.filter_mixed_na",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"id":   knownvalue.StringExact("service_description"),
								"mode": knownvalue.StringExact("is"),
								// Both the real value and the sentinel must be present in state.
								"values": knownvalue.ListExact([]knownvalue.Check{
									knownvalue.StringExact("Compute Engine"),
									knownvalue.StringExact("[Service N/A]"),
								}),
							}),
						}),
					),
				},
			},
			// Step 2: Verify no drift on re-plan.
			// Without the smarter fix, the provider returns values=["Compute Engine"]
			// (API response, without the sentinel), causing perpetual drift.
			{
				Config: testAccReportFilterMixedNAConfig(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportFilterMixedNAConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "filter_mixed_na" {
    name = "test-filter-mixed-na-%d"
    description = "Reproduces API-stripping of N/A sentinel when mixed with real values"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id      = "service_description"
            type    = "fixed"
            inverse = false
            values  = ["Compute Engine", "[Service N/A]"]
            mode    = "is"
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

func testAccReportFilterNAStrippedConfig(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "filter_na_stripped" {
    name = "test-filter-na-stripped-%d"
    description = "Reproduces API stripping N/A filter values"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id      = "csp_primary_domain"
            type    = "fixed"
            inverse = true
            values  = ["[Customer N/A]"]
            mode    = "is"
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_IncludeNullOnlyNoValues tests that a filter with include_null = true
// and NO values is accepted by the API and round-trips without drift.
//
// PR #51575 (fix(analytics): allow include_null and empty values public-api) is
// deployed and the report API accepts this configuration. This test verifies the
// full round-trip: the provider sends include_null=true with an empty values list,
// the API stores it, and the provider reads it back without drift.
//
// If this test fails with a provider inconsistency error, check that report.go
// correctly maps a nil/empty API values list to an empty Terraform list (not null)
// when includeNull=true is set in the filter.
func TestAccReport_IncludeNullOnlyNoValues(t *testing.T) {

	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportIncludeNullOnlyNoValues(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":         knownvalue.StringExact("fixed"),
								"id":           knownvalue.StringExact("service_description"),
								"mode":         knownvalue.StringExact("is"),
								"include_null": knownvalue.Bool(true),
								// values must be empty — no sentinel needed
								"values": knownvalue.ListExact([]knownvalue.Check{}),
							}),
						}),
					),
				},
			},
			// Verify no drift on re-apply
			{
				Config: testAccReportIncludeNullOnlyNoValues(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportIncludeNullOnlyNoValues(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name        = "test-report-include-null-only-%d"
    description = "Report testing include_null=true with no values (pending PR #51575)"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id           = "service_description"
            type         = "fixed"
            inverse      = false
            include_null = true
            values       = []
            mode         = "is"
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_DriftDetection_CustomerPattern tests for drift using the
// customer's exact pattern from ticket 300568: uses metrics (plural list)
// instead of metric (singular), and does NOT set custom_time_range, metric,
// or secondary_time_range. These are the attributes the customer had to add
// ignore_changes blocks for.
func TestAccReport_DriftDetection_CustomerPattern(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportCustomerPattern(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.drift_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			// Drift detection: re-apply same config, expect no changes.
			// This catches drift from API-computed fields like custom_time_range,
			// metric (singular), and secondary_time_range being returned by the
			// API even when the user didn't set them.
			{
				Config: testAccReportCustomerPattern(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportCustomerPattern(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "drift_test" {
    name        = "test-drift-customer-pattern-%d"
    description = "Mirrors customer pattern: uses metrics (plural) and omits optional computed fields"
    config = {
        # Uses metrics (plural list) — NOT metric (singular)
        metrics = [{
            type  = "extended"
            value = "amortized_cost"
        }]
        include_promotional_credits = false
        advanced_analysis = {
            trending_up   = false
            trending_down = false
            not_trending  = false
        }
        aggregation    = "total"
        time_interval  = "month"
        # Intentionally NOT setting: custom_time_range, metric, secondary_time_range
        # These are the attributes the customer had to add ignore_changes for
        data_source    = "billing"
        display_values = "actuals_only"
        layout         = "table"
        currency       = "USD"
    }
}
`, i)
}

// TestAccReport_FolderId verifies that reports can be created inside a folder,
// the folder_id is persisted in state, the report can be moved to root, and
// that re-applying produces no drift.
func TestAccReport_FolderId(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create report inside a folder
			{
				Config: testAccReportInFolder(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.folder_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"doit_report.folder_test", "folder_id",
						"doit_folder.test", "id"),
				),
			},
			// Step 2: Drift check — re-apply same config, expect no changes
			{
				Config: testAccReportInFolder(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Move report to root (folder_id = "root")
			{
				Config: testAccReportInRoot(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.folder_test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.folder_test",
						tfjsonpath.New("folder_id"),
						knownvalue.StringExact("root")),
				},
			},
			// Step 4: Drift check after move
			{
				Config: testAccReportInRoot(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportInFolder(i int) string {
	return fmt.Sprintf(`
resource "doit_folder" "test" {
    name = "tf-acc-report-folder-%d"
}

resource "doit_report" "folder_test" {
    name      = "test-in-folder-%d"
    folder_id = doit_folder.test.id
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
`, i, i)
}

func testAccReportInRoot(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "folder_test" {
    name      = "test-in-folder-%d"
    folder_id = "root"
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

// TestAccReport_DisplaySettings verifies that the display_settings block
// (including theme_id) round-trips without drift on create and update.
func TestAccReport_DisplaySettings(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with display_settings (default theme)
			{
				Config: testAccReportWithDisplaySettings(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.ds_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.ds_test",
						tfjsonpath.New("config").AtMapKey("display_settings").AtMapKey("axis_label_font_size"),
						knownvalue.StringExact("large")),
					statecheck.ExpectKnownValue(
						"doit_report.ds_test",
						tfjsonpath.New("config").AtMapKey("display_settings").AtMapKey("data_label_font_size"),
						knownvalue.StringExact("small")),
					statecheck.ExpectKnownValue(
						"doit_report.ds_test",
						tfjsonpath.New("config").AtMapKey("display_settings").AtMapKey("decimal_precision"),
						knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue(
						"doit_report.ds_test",
						tfjsonpath.New("config").AtMapKey("display_settings").AtMapKey("number_scale"),
						knownvalue.StringExact("millions")),
					statecheck.ExpectKnownValue(
						"doit_report.ds_test",
						tfjsonpath.New("config").AtMapKey("display_settings").AtMapKey("theme_id"),
						knownvalue.StringExact("default")),
				},
			},
			// Step 2: Drift check
			{
				Config: testAccReportWithDisplaySettings(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithDisplaySettings(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "ds_test" {
    name        = "test-display-settings-%d"
    description = "Report testing display_settings block"
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
        display_settings = {
            axis_label_font_size = "large"
            data_label_font_size = "small"
            decimal_precision    = 2
            number_scale         = "millions"
        }
    }
}
`, i)
}

// TestAccReport_DisplaySettingsWithTheme verifies that a custom theme can be
// applied to a report via display_settings.theme_id and that the value
// round-trips without drift. Uses a custom_theme resource as the source.
func TestAccReport_DisplaySettingsWithTheme(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithTheme(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.themed",
							plancheck.ResourceActionCreate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"doit_report.themed", "config.display_settings.theme_id",
						"doit_custom_theme.report_theme", "id"),
				),
			},
			// Drift check
			{
				Config: testAccReportWithTheme(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithTheme(i int) string {
	return fmt.Sprintf(`
resource "doit_custom_theme" "report_theme" {
  name          = "tf-acc-report-theme-%d"
  primary_color = "#FF5733"
  colors = {
    light = ["#FF5733", "#33FF57", "#3357FF"]
    dark  = ["#C70039", "#900C3F", "#581845"]
  }
}

resource "doit_report" "themed" {
    name        = "test-report-themed-%d"
    description = "Report with a custom theme applied"
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
        display_settings = {
            theme_id = doit_custom_theme.report_theme.id
        }
    }
}
`, i, i)
}

// TestAccReport_PartialTimeRange verifies that omitting Optional+Computed
// subfields (here include_current) within a user-specified time_range object
// does not cause errors on Create and does not produce drift on re-apply.
func TestAccReport_PartialTimeRange(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportPartialTimeRange(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("time_range").AtMapKey("mode"),
						knownvalue.StringExact("last")),
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("time_range").AtMapKey("amount"),
						knownvalue.Int64Exact(3)),
					// include_current is omitted — verify it resolves to a known value.
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("time_range").AtMapKey("include_current"),
						knownvalue.NotNull()),
				},
			},
			// Drift check: re-apply same config, expect no changes.
			{
				Config: testAccReportPartialTimeRange(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportPartialTimeRange(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-partial-tr-%d"
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
        time_range = {
          mode   = "last"
          amount = 3
          unit   = "month"
        }
    }
}
`, i)
}

// TestAccReport_EmptyCustomTimeRange verifies that setting custom_time_range = {}
// (both from and to omitted) is rejected at plan time by the config validator.
func TestAccReport_EmptyCustomTimeRange(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportEmptyCustomTimeRange(n),
				ExpectError: regexp.MustCompile(`Empty Custom Time Range`),
			},
		},
	})
}

func testAccReportEmptyCustomTimeRange(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-empty-ctr-%d"
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
        custom_time_range = {}
    }
}
`, i)
}

// TestAccReport_ClearDescription tests that setting a description and then
// removing it from config results in no drift. Validates the
// useEmptyForUnknownWhenConfigNull() plan modifier for issue #233.
func TestAccReport_ClearDescription(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with description set.
			{
				Config: testAccReportWithDescription(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("description"),
						knownvalue.StringExact("initial description")),
				},
			},
			// Step 2: Drift check.
			{
				Config: testAccReportWithDescription(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear description by omitting from config.
			{
				Config: testAccReportClearedDescription(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_report.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Step 4: Drift check — cleared description should produce no drift.
			{
				Config: testAccReportClearedDescription(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithDescription(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name        = "test-minimal-%d"
    description = "initial description"
    labels      = []
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

// testAccReportClearedDescription is the same as testAccReportWithDescription but
// without the description field. All other fields remain identical to isolate the
// clearing behavior to just description.
func testAccReportClearedDescription(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name   = "test-minimal-%d"
    labels = []
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

// TestAccReport_MetricToEmptyMetrics verifies that metrics = [] is rejected at
// plan time. The API silently preserves existing metrics when given an empty
// array, which would cause state inconsistency. The provider's validator
// catches this and returns a clear error.
func TestAccReport_MetricToEmptyMetrics(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportWithEmptyMetrics(n),
				ExpectError: regexp.MustCompile(`Empty Metrics List Not Supported`),
			},
		},
	})
}

func testAccReportWithEmptyMetrics(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "metric_empty_test" {
    name = "test-metric-empty-%d"
    description = "Report to test metric to empty metrics transition"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        metrics        = []
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

// TestAccReport_ClearFilterMode verifies the clearing lifecycle for config.filters[*].mode.
// The API returns null when mode is unset, so omitting it from config clears it.
func TestAccReport_ClearFilterMode(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with mode explicitly set.
			{
				Config: testAccReportWithFilterMode(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.this",
						tfjsonpath.New("config").AtMapKey("filters"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("fixed"),
								"id":     knownvalue.StringExact("cloud_provider"),
								"mode":   knownvalue.StringExact("contains"),
								"values": knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("amazon")}),
							}),
						}),
					),
				},
			},
			// Step 2: Drift check.
			{
				Config: testAccReportWithFilterMode(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Clear mode by omitting it from config.
			{
				Config: testAccReportWithFilterModeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_report.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("doit_report.this", "config.filters.0.mode"),
				),
			},
			// Step 4: Drift check — cleared mode should produce no drift.
			{
				Config: testAccReportWithFilterModeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithFilterMode(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-clear-filter-mode-%d"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id      = "cloud_provider"
            type    = "fixed"
            inverse = false
            mode    = "contains"
            values  = ["amazon"]
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

func testAccReportWithFilterModeCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "this" {
    name = "test-clear-filter-mode-%d"
    config = {
        metric = {
          type  = "basic"
          value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        filters = [
          {
            id      = "cloud_provider"
            type    = "fixed"
            inverse = false
            values  = ["amazon"]
          }
        ]
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// =================================================================----------
// Forecast Settings Acceptance Tests
// =================================================================----------

// TestAccReport_ForecastSettings_Intervals tests forecast settings with intervals.
func TestAccReport_ForecastSettings_Intervals(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithForecastSettingsIntervals(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.forecast_intervals_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-forecast-intervals-%d", n))),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_time_intervals"),
						knownvalue.Int64Exact(12)),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_time_intervals"),
						knownvalue.Int64Exact(12)),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("mode"),
						knownvalue.StringExact("totals")),
				},
			},
			// Drift check: re-apply same config, expect no changes.
			{
				Config: testAccReportWithForecastSettingsIntervals(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithForecastSettingsIntervals(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_intervals_test" {
    name = "test-forecast-intervals-%d"
	description = "Report with forecast settings intervals"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "last"
			unit = "month"
			value = 12
		}
		forecast_settings = {
			future_time_intervals = 12
			historical_time_intervals = 12
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_ForecastSettings_CustomRanges tests forecast settings with custom ranges and timezone offsets preservation.
func TestAccReport_ForecastSettings_CustomRanges(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithForecastSettingsCustomRanges(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.forecast_custom_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.forecast_custom_test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-forecast-custom-%d", n))),
					// Verify original offset timezone strings are preserved perfectly
					statecheck.ExpectKnownValue(
						"doit_report.forecast_custom_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range").AtMapKey("from"),
						knownvalue.StringExact("2024-02-02T00:00:00+02:00")),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_custom_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range").AtMapKey("to"),
						knownvalue.StringExact("2024-08-02T00:00:00+02:00")),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_custom_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range").AtMapKey("from"),
						knownvalue.StringExact("2023-01-01T00:00:00-05:00")),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_custom_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range").AtMapKey("to"),
						knownvalue.StringExact("2023-12-31T23:59:59-05:00")),
				},
			},
			// Drift check: re-apply same config, expect no changes.
			{
				Config: testAccReportWithForecastSettingsCustomRanges(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithForecastSettingsCustomRanges(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_custom_test" {
    name = "test-forecast-custom-%d"
	description = "Report with forecast settings custom date ranges"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "custom"
			unit = "day"
		}
		custom_time_range = {
			from = "2023-01-01T00:00:00-05:00"
			to   = "2023-12-31T23:59:59-05:00"
		}
		forecast_settings = {
			future_custom_date_range = {
				from = "2024-02-02T00:00:00+02:00"
				to   = "2024-08-02T00:00:00+02:00"
			}
			historical_custom_date_range = {
				from = "2023-01-01T00:00:00-05:00"
				to   = "2023-12-31T23:59:59-05:00"
			}
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_ForecastSettings_Lifecycle tests updating a forecast settings resource through its lifecycle (intervals -> custom -> intervals -> completely removed).
func TestAccReport_ForecastSettings_Lifecycle(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with intervals.
			{
				Config: testAccReportWithForecastSettingsIntervals(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_time_intervals"),
						knownvalue.Int64Exact(12)),
				},
			},
			// Drift check after Create.
			{
				Config: testAccReportWithForecastSettingsIntervals(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 2: Switch to custom date ranges.
			{
				Config: testAccReportWithForecastSettingsCustomRangesUpdate(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range").AtMapKey("from"),
						knownvalue.StringExact("2024-02-02T00:00:00+03:00")),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range").AtMapKey("to"),
						knownvalue.StringExact("2024-08-02T00:00:00+03:00")),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range").AtMapKey("from"),
						knownvalue.StringExact("2023-01-01T00:00:00-08:00")),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range").AtMapKey("to"),
						knownvalue.StringExact("2023-12-31T23:59:59-08:00")),
				},
			},
			// Drift check after switching to custom.
			{
				Config: testAccReportWithForecastSettingsCustomRangesUpdate(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Switch back to intervals.
			{
				Config: testAccReportWithForecastSettingsIntervals(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_time_intervals"),
						knownvalue.Int64Exact(12)),
					// Switching back to intervals must clear the custom ranges; if retained
					// they would be sent to the API and take precedence over the intervals.
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_custom_date_range"),
						knownvalue.Null()),
					statecheck.ExpectKnownValue(
						"doit_report.forecast_intervals_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("historical_custom_date_range"),
						knownvalue.Null()),
				},
			},
			// Drift check after switching back to intervals.
			{
				Config: testAccReportWithForecastSettingsIntervals(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 4: Remove forecast_settings entirely.
			{
				Config: testAccReportWithForecastSettingsRemoved(n),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("doit_report.forecast_intervals_test", "config.forecast_settings"),
				),
			},
			// Drift check after removal.
			{
				Config: testAccReportWithForecastSettingsRemoved(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithForecastSettingsCustomRangesUpdate(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_intervals_test" {
    name = "test-forecast-intervals-%d"
	description = "Report with forecast settings intervals updated to custom ranges"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "last"
			unit = "month"
			value = 12
		}
		forecast_settings = {
			future_custom_date_range = {
				from = "2024-02-02T00:00:00+03:00"
				to   = "2024-08-02T00:00:00+03:00"
			}
			historical_custom_date_range = {
				from = "2023-01-01T00:00:00-08:00"
				to   = "2023-12-31T23:59:59-08:00"
			}
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

func testAccReportWithForecastSettingsRemoved(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_intervals_test" {
    name = "test-forecast-intervals-%d"
	description = "Report with forecast settings completely removed"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "custom"
			unit = "day"
		}
		custom_time_range = {
			from = "2023-01-01T00:00:00-05:00"
			to   = "2023-12-31T23:59:59-05:00"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_ForecastSettings_Invalid tests config validation for invalid forecast settings.
func TestAccReport_ForecastSettings_Invalid(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportWithForecastSettingsInvalidTimestamp(n),
				ExpectError: regexp.MustCompile(`Invalid RFC3339 Timestamp`),
			},
			{
				Config:      testAccReportWithForecastSettingsEmptyCustomDateRange(n),
				ExpectError: regexp.MustCompile(`Empty Future Custom Date Range`),
			},
		},
	})
}

func testAccReportWithForecastSettingsInvalidTimestamp(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_invalid" {
    name = "test-forecast-invalid-%d"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "custom"
			unit = "day"
		}
		custom_time_range = {
			from = "2023-01-01T00:00:00Z"
			to   = "2023-12-31T23:59:59Z"
		}
		forecast_settings = {
			future_custom_date_range = {
				from = "invalid-timestamp"
				to   = "2024-08-02T00:00:00Z"
			}
			historical_custom_date_range = {
				from = "2023-01-01T00:00:00Z"
				to   = "2023-12-31T23:59:59Z"
			}
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

func testAccReportWithForecastSettingsEmptyCustomDateRange(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_empty" {
    name = "test-forecast-empty-%d"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "custom"
			unit = "day"
		}
		custom_time_range = {
			from = "2023-01-01T00:00:00Z"
			to   = "2023-12-31T23:59:59Z"
		}
		forecast_settings = {
			future_custom_date_range = {}
			historical_custom_date_range = {
				from = "2023-01-01T00:00:00Z"
				to   = "2023-12-31T23:59:59Z"
			}
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_ForecastSettings_GroupingModeOnly tests that forecast_settings with mode = "grouping" only is preserved correctly.
func TestAccReport_ForecastSettings_GroupingModeOnly(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportWithForecastSettingsGroupingModeOnly(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.forecast_grouping_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("mode"),
						knownvalue.StringExact("grouping")),
				},
			},
			// Drift check.
			{
				Config: testAccReportWithForecastSettingsGroupingModeOnly(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithForecastSettingsGroupingModeOnly(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_grouping_test" {
    name = "test-forecast-grouping-%d"
	description = "Report with forecast settings grouping mode only"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "last"
			unit = "month"
			value = 12
		}
		forecast_settings = {
			mode = "grouping"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

func TestAccReport_ForecastSettings_Conflict(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportWithForecastSettingsConflict(n),
				ExpectError: regexp.MustCompile(`Conflicting Forecast Configuration`),
			},
		},
	})
}

func testAccReportWithForecastSettingsConflict(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "forecast_conflict" {
    name = "test-forecast-conflict-%d"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "last"
			unit = "month"
			value = 12
		}
		advanced_analysis = {
			forecast = false
		}
		forecast_settings = {
			future_time_intervals = 6
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_ForecastSettings_RetainedForecastFalse creates a non-forecast report
// (advanced_analysis set, forecast omitted so the API persists false), then adds
// forecast_settings while still omitting forecast. It verifies forecasting is enabled
// and the result is stable — no apply error and no drift.
func TestAccReport_ForecastSettings_RetainedForecastFalse(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: non-forecast report; forecast omitted from advanced_analysis.
			{
				Config: testAccReportRetainedForecastFalseNoSettings(n),
				ConfigStateChecks: []statecheck.StateCheck{
					// The API persists forecast=false for a non-forecast report; this is
					// what Optional+Computed then retains into the next plan.
					statecheck.ExpectKnownValue(
						"doit_report.retained_forecast_test",
						tfjsonpath.New("config").AtMapKey("advanced_analysis").AtMapKey("forecast"),
						knownvalue.Bool(false)),
				},
			},
			// Drift check after create.
			{
				Config: testAccReportRetainedForecastFalseNoSettings(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 2: add forecast_settings, still omitting forecast.
			{
				Config: testAccReportRetainedForecastFalseWithSettings(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.retained_forecast_test",
						tfjsonpath.New("config").AtMapKey("forecast_settings").AtMapKey("future_time_intervals"),
						knownvalue.Int64Exact(12)),
					// Adding forecast_settings enables forecasting: forecast resolves to true.
					statecheck.ExpectKnownValue(
						"doit_report.retained_forecast_test",
						tfjsonpath.New("config").AtMapKey("advanced_analysis").AtMapKey("forecast"),
						knownvalue.Bool(true)),
				},
			},
			// Drift check after adding settings.
			{
				Config: testAccReportRetainedForecastFalseWithSettings(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportRetainedForecastFalseNoSettings(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "retained_forecast_test" {
    name = "test-retained-forecast-%d"
	description = "Non-forecast report; forecast omitted from advanced_analysis"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "last"
			unit = "month"
			value = 12
		}
		advanced_analysis = {
			not_trending = true
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

func testAccReportRetainedForecastFalseWithSettings(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "retained_forecast_test" {
    name = "test-retained-forecast-%d"
	description = "Forecast settings added while forecast still omitted"
	config = {
		metric = {
		  type  = "basic"
		  value = "cost"
		}
		aggregation   = "total"
		time_interval = "month"
		time_range = {
			mode = "last"
			unit = "month"
			value = 12
		}
		advanced_analysis = {
			not_trending = true
		}
		forecast_settings = {
			future_time_intervals = 12
			mode = "totals"
		}
		data_source    = "billing"
		display_values = "actuals_only"
		currency       = "USD"
		layout         = "table"
	}
}
`, i)
}

// TestAccReport_MetricFilterOperand covers the full lifecycle of the new
// config.metric_filter.operand attribute (Optional+Computed, Default "single_value").
func TestAccReport_MetricFilterOperand(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create with operand = series_total.
			{
				Config: testAccReportOperand(n, `operand = "series_total"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.operand",
						tfjsonpath.New("config").AtMapKey("metric_filter").AtMapKey("operand"),
						knownvalue.StringExact("series_total")),
				},
			},
			// Step 2: drift check.
			{
				Config: testAccReportOperand(n, `operand = "series_total"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Step 3: import while operand is at its non-default value, verifying
			// import reconstructs series_total.
			{
				ResourceName:      "doit_report.operand",
				ImportState:       true,
				ImportStateVerify: true,
				// config.metric and config.metrics are two views of one API field.
				// Import has no config to tell which one is in use, so it populates the
				// canonical metrics and leaves the deprecated metric mirror null (see
				// mapReportToModel). Trailing dots keep config.metric_filter.* matched.
				ImportStateVerifyIgnore: []string{"config.metric.", "config.metrics."},
			},
			// Step 4: omit operand -> resolves to the schema default single_value (Update).
			{
				Config: testAccReportOperand(n, ``),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_report.operand", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.operand",
						tfjsonpath.New("config").AtMapKey("metric_filter").AtMapKey("operand"),
						knownvalue.StringExact("single_value")),
				},
			},
			// Step 5: drift check on the defaulted value.
			{
				Config: testAccReportOperand(n, ``),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_LimitAggregation covers the full lifecycle of the new
// config.limit_aggregation attribute (Optional+Computed, Default "none").
func TestAccReport_LimitAggregation(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create with limit_aggregation = top (a group limit is active).
			{
				Config: testAccReportLimitAggregation(n, `limit_aggregation = "top"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.limitagg",
						tfjsonpath.New("config").AtMapKey("limit_aggregation"),
						knownvalue.StringExact("top")),
				},
			},
			// Step 2: drift check.
			{
				Config: testAccReportLimitAggregation(n, `limit_aggregation = "top"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Step 3: import while limit_aggregation is at its non-default value,
			// verifying import reconstructs top.
			{
				ResourceName:      "doit_report.limitagg",
				ImportState:       true,
				ImportStateVerify: true,
				// config.metric and config.metrics are two views of one API field.
				// Import has no config to tell which one is in use, so it populates the
				// canonical metrics and leaves the deprecated metric mirror null (see
				// mapReportToModel). Trailing dots keep config.metric_filter.* matched.
				ImportStateVerifyIgnore: []string{"config.metric.", "config.metrics."},
			},
			// Step 4: update to none.
			{
				Config: testAccReportLimitAggregation(n, `limit_aggregation = "none"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_report.limitagg", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.limitagg",
						tfjsonpath.New("config").AtMapKey("limit_aggregation"),
						knownvalue.StringExact("none")),
				},
			},
			// Step 5: omit limit_aggregation -> resolves to the default none; no drift
			// versus the explicit none from step 4.
			{
				Config: testAccReportLimitAggregation(n, ``),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.limitagg",
						tfjsonpath.New("config").AtMapKey("limit_aggregation"),
						knownvalue.StringExact("none")),
				},
			},
		},
	})
}

// TestAccReport_LimitAggregationInvalidDisplayValues verifies the API constraint
// that limit_aggregation must be none when display_values is not actuals_only.
func TestAccReport_LimitAggregationInvalidDisplayValues(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportLimitAggregationBadView(n),
				ExpectError: regexp.MustCompile(`INVALID_REMAINING_RESULTS_MODE`),
			},
		},
	})
}

// TestAccReport_LimitByChange covers the full lifecycle of the new
// config.limit_by_change nested object: create, drift, import, and update.
func TestAccReport_LimitByChange(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create with a percentage/>=/[50] filter.
			{
				Config: testAccReportLimitByChange(n, "percentage", ">=", "[50]", "false"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("change_type"),
						knownvalue.StringExact("percentage")),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("operator"),
						knownvalue.StringExact(">=")),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("include_incomplete_data"),
						knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("values").AtSliceIndex(0),
						knownvalue.Float64Exact(50)),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("metric").AtMapKey("value"),
						knownvalue.StringExact("cost")),
				},
			},
			// Step 2: drift check.
			{
				Config: testAccReportLimitByChange(n, "percentage", ">=", "[50]", "false"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Step 3: import + verify.
			{
				ResourceName:      "doit_report.lbc",
				ImportState:       true,
				ImportStateVerify: true,
				// config.metric and config.metrics are two views of one API field.
				// Import has no config to tell which one is in use, so it populates the
				// canonical metrics and leaves the deprecated metric mirror null (see
				// mapReportToModel). Trailing dots keep config.metric_filter.* matched.
				ImportStateVerifyIgnore: []string{"config.metric.", "config.metrics."},
			},
			// Step 4: update to absolute/between/[10,90], include_incomplete_data = true.
			{
				Config: testAccReportLimitByChange(n, "absolute", "between", "[10, 90]", "true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("doit_report.lbc", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("change_type"),
						knownvalue.StringExact("absolute")),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("operator"),
						knownvalue.StringExact("between")),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("include_incomplete_data"),
						knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("values"),
						knownvalue.ListSizeExact(2)),
				},
			},
			// Step 5: drift check after update.
			{
				Config: testAccReportLimitByChange(n, "absolute", "between", "[10, 90]", "true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_LimitByChangeOmitted verifies that omitting limit_by_change yields
// a null value that is stable across re-apply (matches the metric_filter precedent).
func TestAccReport_LimitByChangeOmitted(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportMinimal(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.this",
						tfjsonpath.New("config").AtMapKey("limit_by_change"),
						knownvalue.Null()),
				},
			},
			{
				Config: testAccReportMinimal(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_ThreeLimitsNotAllowed verifies the API constraint that a report
// may configure at most two of metric_filter, limit_by_change and a group limit.
func TestAccReport_ThreeLimitsNotAllowed(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportThreeLimits(n),
				ExpectError: regexp.MustCompile(`THREE_LIMITS_NOT_ALLOWED`),
			},
		},
	})
}

func testAccReportOperand(i int, operandLine string) string {
	return fmt.Sprintf(`
resource "doit_report" "operand" {
    name = "test-operand-%d"
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
            operator = "gt"
            values   = [100]
            %s
        }
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i, operandLine)
}

func testAccReportLimitAggregation(i int, limitAggLine string) string {
	return fmt.Sprintf(`
resource "doit_report" "limitagg" {
    name = "test-limitagg-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        %s
        group = [
            {
                id   = "sku_description"
                type = "fixed"
                limit = {
                    value  = 5
                    sort   = "desc"
                    metric = {
                        type  = "basic"
                        value = "cost"
                    }
                }
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
`, i, limitAggLine)
}

func testAccReportLimitAggregationBadView(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "limitagg_badview" {
    name = "test-limitagg-badview-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        limit_aggregation = "top"
        group = [
            {
                id   = "sku_description"
                type = "fixed"
                limit = {
                    value  = 5
                    sort   = "desc"
                    metric = {
                        type  = "basic"
                        value = "cost"
                    }
                }
            }
        ]
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "absolute_and_percentage"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

func testAccReportLimitByChange(i int, changeType, operator, values, includeIncomplete string) string {
	return fmt.Sprintf(`
resource "doit_report" "lbc" {
    name = "test-lbc-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        limit_by_change = {
            metric = {
                type  = "basic"
                value = "cost"
            }
            change_type             = %q
            operator                = %q
            values                  = %s
            include_incomplete_data = %s
        }
        aggregation    = "total"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i, changeType, operator, values, includeIncomplete)
}

func testAccReportThreeLimits(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "three_limits" {
    name = "test-three-limits-%d"
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
            operator = "gt"
            values   = [1]
        }
        limit_by_change = {
            metric = {
                type  = "basic"
                value = "cost"
            }
            change_type             = "percentage"
            operator                = ">="
            values                  = [50]
            include_incomplete_data = false
        }
        group = [
            {
                id   = "sku_description"
                type = "fixed"
                limit = {
                    value  = 5
                    sort   = "desc"
                    metric = {
                        type  = "basic"
                        value = "cost"
                    }
                }
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

// TestAccReport_MetricFieldsRequired asserts the DoiT API requirement that
// every configured metric object must set both type and value. This used to be
// surfaced by a hand-written plan-time validator (reportMetricFieldsValidator,
// added because the upstream OpenAPI ExternalMetric schema listed no required
// fields); the upstream spec now marks both required directly, so the
// generated schema itself enforces this — Terraform's config decoder rejects
// the omission before the provider is ever called, and the validator was
// removed. This covers the shared metric handling consistently across
// contexts: limit_by_change.metric, metric_filter.metric and the top-level
// metric.
func TestAccReport_MetricFieldsRequired(t *testing.T) {
	// HCL's diagnostic renderer word-wraps the message, so "attribute" and the
	// quoted field name can land on different lines — match across the wrap.
	missingRequiredField := regexp.MustCompile(`attribute\s+"(type|value)" is required`)

	cases := []struct {
		name   string
		config func(int) string
	}{
		// limit_by_change.metric omitting type — the exact scenario Copilot flagged.
		{"limit_by_change_metric_missing_type", testAccReportLimitByChangeMetricMissingType},
		// metric_filter.metric omitting type — same shared metric handling.
		{"metric_filter_metric_missing_type", testAccReportMetricFilterMetricMissingType},
		// top-level metric omitting value — asserts value is required too.
		{"metric_missing_value", testAccReportMetricMissingValue},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := acctest.RandInt()
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config:      tc.config(n),
						ExpectError: missingRequiredField,
					},
				},
			})
		})
	}
}

func testAccReportLimitByChangeMetricMissingType(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "lbc_no_type" {
    name = "test-lbc-no-type-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        limit_by_change = {
            metric = {
                value = "cost"
            }
            change_type             = "percentage"
            operator                = ">="
            values                  = [50]
            include_incomplete_data = false
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

func testAccReportMetricFilterMetricMissingType(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "mf_no_type" {
    name = "test-mf-no-type-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        metric_filter = {
            metric = {
                value = "cost"
            }
            operator = "gt"
            values   = [100]
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

func testAccReportMetricMissingValue(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "metric_no_value" {
    name = "test-metric-no-value-%d"
    config = {
        metric = {
            type = "basic"
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

// TestAccReport_GroupRemoval_Permadiff documents a KNOWN REGRESSION, deliberately
// skipped rather than deleted so it is not rediscovered from scratch.
//
// config.group is Optional+Computed and now has a Required descendant
// (group[*].limit.metric.type), so removing a group block that carries a
// limit.metric hits the same Terraform Core rule described in
// useNullForUnconfiguredMetricMirror: Core proposes null, the plan never matches
// prior, and it does not converge. Measured behaviour:
//
//   - Omitting the group block: apply "succeeds" (the API PATCH merge treats an
//     omitted group as no change), Read puts the group back, and every subsequent
//     plan diffs again — forever.
//   - group = [] alone does not clear it either, when the group carries a limit:
//     the API returns 400 {"field":"filters","message":"filter id is not listed
//     in the rows field: fixed:cloud_provider"}. Sending {"group":[],"filters":[]}
//     in the same request DOES clear it (200). A group with no limit clears with
//     group:[] alone. So a top/bottom limit implies a server-side filter on the
//     grouped dimension that is orphaned when the group goes away — and that
//     filter is NOT visible in GET .../config (it reports filters: null), so the
//     provider cannot drop just the offending filter; clearing all filters is the
//     only lever, which would wipe filters set outside Terraform.
//
// Scope: only groups carrying a limit are affected. Without a limit every
// descendant of group[*] is Optional+Computed (id, type), so
// optionalValueNotComputable stays false and removal behaves as before. The
// with-limit permadiff was measured; the without-limit case is inferred from the
// schema and has not been exercised end to end.
//
// Before the metric leaves became Required every descendant of group was Computed,
// so removal was silently preserved with no drift (Category B). Fixing it is a
// product decision — genuine clearing (now known to be possible, but needs the
// filters coupling resolved), Category C replace-on-removal (destroy+creates the
// report, changing its ID), or dropping it from state (misrepresents real user
// data). The API team is investigating the filters coupling; deferred until that
// lands.
func TestAccReport_GroupRemoval_Permadiff(t *testing.T) {
	t.Skip("known regression: removing a group block with limit.metric does not converge; see doc comment")
}

// TestAccReport_Metric_MirrorSwitchBackAndForth covers the reverse of
// TestAccReport_Metric_UnconfiguredMirrorNotStored: metrics -> metric -> metrics.
// Whichever mirror the practitioner configures must survive and the other must
// stay out of state, in both directions. The provider can always tell them apart
// because the plan modifier keys off req.ConfigValue, which Terraform does supply
// at plan time — prior state alone is ambiguous (the API returns both populated),
// which is why this cannot be handled by a state upgrader.
func TestAccReport_Metric_MirrorSwitchBackAndForth(t *testing.T) {
	n := acctest.RandInt()

	metricsOnly := []statecheck.StateCheck{
		statecheck.ExpectKnownValue("doit_report.metric_clear",
			tfjsonpath.New("config").AtMapKey("metric"), knownvalue.Null()),
		statecheck.ExpectKnownValue("doit_report.metric_clear",
			tfjsonpath.New("config").AtMapKey("metrics").AtSliceIndex(0).AtMapKey("value"),
			knownvalue.StringExact("usage")),
	}
	metricOnly := []statecheck.StateCheck{
		statecheck.ExpectKnownValue("doit_report.metric_clear",
			tfjsonpath.New("config").AtMapKey("metric").AtMapKey("value"),
			knownvalue.StringExact("cost")),
		statecheck.ExpectKnownValue("doit_report.metric_clear",
			tfjsonpath.New("config").AtMapKey("metrics"),
			knownvalue.ListExact([]knownvalue.Check{})),
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Start on the canonical metrics list.
			{
				Config:            testAccReportMetricCleared(n),
				ConfigStateChecks: metricsOnly,
			},
			// Switch to the deprecated singular metric.
			{
				Config:            testAccReportWithSingularMetric(n),
				ConfigStateChecks: metricOnly,
			},
			{
				Config: testAccReportWithSingularMetric(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// ...and back again.
			{
				Config:            testAccReportMetricCleared(n),
				ConfigStateChecks: metricsOnly,
			},
			{
				Config: testAccReportMetricCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_Metric_UpgradeFromBothMirrorsInState is the upgrade-safety test.
//
// Provider versions before the metric type/value leaves became Required stored
// BOTH mirrors in state, because the API returns both populated regardless of
// which one was configured. Existing practitioners therefore upgrade with an
// ambiguous state — nothing that sees only prior state (a state upgrader
// included) can tell which mirror was configured.
//
// The plan modifier resolves it from config, so the outcome is correct in both
// directions. The cost is one noisy-but-convergent apply: Terraform Core proposes
// null for the now-unconfigured mirror, which makes plan != prior and cascades the
// rest of config to "known after apply" for that single plan. This test pins that
// the upgrade converges and that the configured mirror is preserved.
func TestAccReport_Metric_UpgradeFromBothMirrorsInState(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:               testAccPreCheckFunc(t),
		TerraformVersionChecks: testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create with the last release, which writes BOTH mirrors.
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"doit": {Source: "doitintl/doit", VersionConstraint: "1.6.0"},
				},
				Config: testAccReportWithSingularMetric(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.metric_clear",
						tfjsonpath.New("config").AtMapKey("metric").AtMapKey("value"),
						knownvalue.StringExact("cost")),
					statecheck.ExpectKnownValue("doit_report.metric_clear",
						tfjsonpath.New("config").AtMapKey("metrics").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("cost")),
				},
			},
			// Step 2: same config, current provider. One convergent apply; the
			// configured mirror survives and the echoed one drops out of state.
			{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				Config:                   testAccReportWithSingularMetric(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("doit_report.metric_clear",
						tfjsonpath.New("config").AtMapKey("metric").AtMapKey("value"),
						knownvalue.StringExact("cost")),
					statecheck.ExpectKnownValue("doit_report.metric_clear",
						tfjsonpath.New("config").AtMapKey("metrics"),
						knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Step 3: the upgrade has settled — no permadiff.
			{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				Config:                   testAccReportWithSingularMetric(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccReport_NestedMetricOmitted covers the nested metric containers whose
// requirement was previously only enforced by the API.
//
// These were originally added as probes: the API rejected an omitted
// `metric_filter.metric` / `group[*].limit.metric` with a 400 at apply time
// ("metric can not be null"), while the spec left both optional. The API team
// confirmed the requirement and the spec now marks `metric` required on both
// `Limit` and `ExternalConfigMetricFilter` (which also gained `operator` and
// `values`), so the generated schema enforces all of it at plan time and the
// cryptic apply-time 400 is gone.
//
// This is also why neither container is exposed to the phantom diff that
// config.metric/config.metrics needed handling for: the API never lets such a
// report exist, so state can never hold an unconfigured-but-echoed metric there.
func TestAccReport_NestedMetricOmitted(t *testing.T) {
	// Terraform's own config decoder rejects these now, before the provider is
	// called. HCL word-wraps diagnostics, so tolerate a wrap before the name.
	missingRequiredField := regexp.MustCompile(`attribute\s+"(metric|operator|values)" is required`)

	cases := []struct {
		name   string
		config func(int) string
	}{
		{"metric_filter_metric_omitted", testAccReportMetricFilterMetricOmitted},
		{"group_limit_metric_omitted", testAccReportGroupLimitMetricOmitted},
		{"metric_filter_operator_omitted", testAccReportMetricFilterOperatorOmitted},
		{"metric_filter_values_omitted", testAccReportMetricFilterValuesOmitted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := acctest.RandInt()
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config:      tc.config(n),
						ExpectError: missingRequiredField,
					},
				},
			})
		})
	}
}

func testAccReportMetricFilterMetricOmitted(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "mf_metric_omitted" {
    name = "test-mf-metric-omitted-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        metric_filter = {
            operator = "gt"
            values   = [1]
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

func testAccReportMetricFilterOperatorOmitted(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "mf_operator_omitted" {
    name = "test-mf-operator-omitted-%d"
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
            values = [1]
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

func testAccReportMetricFilterValuesOmitted(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "mf_values_omitted" {
    name = "test-mf-values-omitted-%d"
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
            operator = "gt"
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

func testAccReportGroupLimitMetricOmitted(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "grp_limit_metric_omitted" {
    name = "test-grp-limit-metric-omitted-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        group = [{
            id   = "cloud_provider"
            type = "fixed"
            limit = {
                value = 5
                sort  = "desc"
            }
        }]
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

// TestAccReport_LimitByChangeNotClearable proves config.limit_by_change cannot be
// cleared in place (an in-place update perpetually drifts, since the PATCH merge
// retains the stored value), so ModifyPlan forces a destroy+create (Category C)
// when it is removed.
func TestAccReport_LimitByChangeNotClearable(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.lbc",
		tfjsonpath.New("config").AtMapKey("limit_by_change").AtMapKey("operator"),
		knownvalue.StringExact(">="))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH limit_by_change.
			{
				Config:            testAccReportLimitByChange(n, "percentage", ">=", "[50]", "false"),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: omit limit_by_change — removal forces a replacement, and the
			// recreated report has no limit_by_change.
			{
				Config: testAccReportLimitByChangeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_report.lbc",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.lbc",
						tfjsonpath.New("config").AtMapKey("limit_by_change"),
						knownvalue.Null()),
				},
			},
			// Step 3: drift check after replacement.
			{
				Config: testAccReportLimitByChangeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// testAccReportLimitByChangeCleared is testAccReportLimitByChange with the
// limit_by_change block removed but the same resource address and name, so it can
// be used as a follow-up step to test omitting the attribute.
func testAccReportLimitByChangeCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "lbc" {
    name = "test-lbc-%d"
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

// TestAccReport_Count_Lifecycle exercises the full lifecycle of the config.count
// nested object: create with aggregation="count", drift, update the counted field,
// drift, import, then transition aggregation away and drop count.
func TestAccReport_Count_Lifecycle(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: Create with count.
			{
				Config: testAccReportCount(n, "service_description", "fixed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.count_test",
							plancheck.ResourceActionCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.count_test",
						tfjsonpath.New("config").AtMapKey("count").AtMapKey("id"),
						knownvalue.StringExact("service_description")),
					statecheck.ExpectKnownValue(
						"doit_report.count_test",
						tfjsonpath.New("config").AtMapKey("count").AtMapKey("type"),
						knownvalue.StringExact("fixed")),
				},
			},
			// Drift check after Create.
			{
				Config: testAccReportCount(n, "service_description", "fixed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 2: Update the counted field.
			{
				Config: testAccReportCount(n, "sku_description", "fixed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"doit_report.count_test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.count_test",
						tfjsonpath.New("config").AtMapKey("count").AtMapKey("id"),
						knownvalue.StringExact("sku_description")),
				},
			},
			// Drift check after Update.
			{
				Config: testAccReportCount(n, "sku_description", "fixed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: Import mid-lifecycle.
			{
				ResourceName:      "doit_report.count_test",
				ImportState:       true,
				ImportStateVerify: true,
				// config.metric and config.metrics are two views of one API field.
				// Import has no config to tell which one is in use, so it populates the
				// canonical metrics and leaves the deprecated metric mirror null (see
				// mapReportToModel). Trailing dots keep config.metric_filter.* matched.
				ImportStateVerifyIgnore: []string{"config.metric.", "config.metrics."},
			},
		},
	})
}

// TestAccReport_Count_Omitted verifies that omitting count (non-count aggregation)
// leaves config.count null and produces no drift.
func TestAccReport_Count_Omitted(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportCountOmitted(n),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.count_test",
						tfjsonpath.New("config").AtMapKey("count"),
						knownvalue.Null()),
				},
			},
			// Drift check.
			{
				Config: testAccReportCountOmitted(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccReport_Count_InvalidAggregation verifies the cross-field validator rejects
// count when aggregation is not "count".
func TestAccReport_Count_InvalidAggregation(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportCountInvalidAggregation(n),
				ExpectError: regexp.MustCompile(`Invalid Count Configuration`),
			},
		},
	})
}

func testAccReportCount(i int, id, ctype string) string {
	return fmt.Sprintf(`
resource "doit_report" "count_test" {
    name = "test-count-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        aggregation    = "count"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        count = {
            id   = %q
            type = %q
        }
    }
}
`, i, id, ctype)
}

func testAccReportCountOmitted(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "count_test" {
    name = "test-count-%d"
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

func testAccReportCountInvalidAggregation(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "count_invalid" {
    name = "test-count-invalid-%d"
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
        count = {
            id   = "service_description"
            type = "fixed"
        }
    }
}
`, i)
}

// TestAccReport_Count_NoAggregation verifies that count with an omitted aggregation
// is rejected at plan time. The API does not default aggregation to "count" when
// omitted (verified: it returns 400 "count field is only valid when aggregation is
// 'count'"), so the validator must catch this before apply.
func TestAccReport_Count_NoAggregation(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportCountNoAggregation(n),
				ExpectError: regexp.MustCompile(`Invalid Count Configuration`),
			},
		},
	})
}

func testAccReportCountNoAggregation(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "count_noagg" {
    name = "test-count-noagg-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
        count = {
            id   = "service_description"
            type = "fixed"
        }
    }
}
`, i)
}

// TestAccReport_Count_RemovalForcesReplace verifies that removing count from config
// forces a destroy+create (RequiresReplace), rather than perpetual drift or an API
// error. count cannot be cleared in place (PATCH merge + no null representation), so
// ModifyPlan surfaces its removal as a replacement.
func TestAccReport_Count_RemovalForcesReplace(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Create with count (aggregation = "count").
			{
				Config: testAccReportCount(n, "service_description", "fixed"),
			},
			// Remove count and switch aggregation to "total": expect a replacement,
			// and the recreated report has no count.
			{
				Config: testAccReportCountOmitted(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_report.count_test",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.count_test",
						tfjsonpath.New("config").AtMapKey("count"),
						knownvalue.Null()),
				},
			},
			// Drift check after replacement.
			{
				Config: testAccReportCountOmitted(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccReport_Count_RequiredWhenAggregationCount verifies that aggregation =
// "count" without a count block is rejected at plan time (the API has no default
// counted field and returns a 400 otherwise).
func TestAccReport_Count_RequiredWhenAggregationCount(t *testing.T) {
	n := acctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      testAccReportCountAggNoBlock(n),
				ExpectError: regexp.MustCompile(`Missing Count Configuration`),
			},
		},
	})
}

func testAccReportCountAggNoBlock(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "count_aggnoblock" {
    name = "test-count-aggnoblock-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        aggregation    = "count"
        time_interval  = "month"
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// -----------------------------------------------------------------------------
// clearableattr (issue #233): removal behavior for the nested Optional+Computed
// objects the linter flags in config.
//
// Each object is serialized in the update request as *T with `omitempty` (see
// ExternalConfig in models_gen.go), and the report update is a PATCH that merges
// config with no null representation. None can be cleared in place; the observed
// removal behavior splits into two groups:
//
// Silently preserved (Category B behavior — removal is idempotent, the prior value
// sticks with no drift and no error; left as-is, forcing a replace would be
// gratuitously destructive):
//   - advanced_analysis, display_settings, time_range, secondary_time_range
//   - metric (deprecated): mirrors metrics[0] rather than clearing
//   - forecast_settings.future/historical_custom_date_range: retained when dropped
//     while keeping forecast_settings (they only clear when time_intervals are
//     supplied instead, via the API's server-side mutual exclusion — see
//     TestAccReport_ForecastSettings_Lifecycle)
//
// Harmful — remediated with Category C replace-on-clear in reportResource.ModifyPlan
// (requiresReplaceWhenCleared), so removing the block destroys+recreates:
//   - limit_by_change: would otherwise perpetually drift
//   - metric_filter (+ its required nested metric): the update would send an empty
//     metricFilter and the API would reject it (400)
//   - custom_time_range: the stale range would conflict with a changed
//     time_range.mode and the API would reject the update (400)
//
// (config.count is the pre-existing Category C case; see TestAccReport_Count_*.)
// -----------------------------------------------------------------------------

// TestAccReport_AdvancedAnalysis_NotClearable proves config.advanced_analysis
// cannot be cleared: removing it is silently preserved (idempotent, no drift) —
// the prior value sticks in state.
func TestAccReport_AdvancedAnalysis_NotClearable(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.aa_clear",
		tfjsonpath.New("config").AtMapKey("advanced_analysis").AtMapKey("trending_up"),
		knownvalue.Bool(true))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH advanced_analysis.
			{
				Config:            testAccReportWithAdvancedAnalysis(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: omit advanced_analysis. The API silently preserves it, so the
			// removal applies idempotently (no error) and the value stays in state.
			{
				Config:            testAccReportAdvancedAnalysisCleared(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 3: drift check — the retained value produces no diff, confirming
			// removal is silently ignored rather than causing a permadiff.
			{
				Config: testAccReportAdvancedAnalysisCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithAdvancedAnalysis(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "aa_clear" {
    name = "test-aa-clear-%d"
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
        advanced_analysis = {
            trending_up   = true
            trending_down = true
            not_trending  = true
            forecast      = false
        }
    }
}
`, i)
}

func testAccReportAdvancedAnalysisCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "aa_clear" {
    name = "test-aa-clear-%d"
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

// TestAccReport_CustomTimeRange_RemovalForcesReplace proves config.custom_time_range
// cannot be cleared in place. It is only meaningful with time_range.mode = "custom";
// an in-place update that switches the mode to "last" and drops the block would
// leave the stale custom range in the merged config and the API would reject it
// (400). ModifyPlan forces a destroy+create (Category C) when it is removed.
func TestAccReport_CustomTimeRange_RemovalForcesReplace(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.ctr_clear",
		tfjsonpath.New("config").AtMapKey("custom_time_range").AtMapKey("from"),
		knownvalue.StringExact("2024-01-01T00:00:00Z"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH a custom time range.
			{
				Config:            testAccReportWithCustomTimeRange(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: switch to a relative range and drop custom_time_range —
			// removal forces a replacement, and the recreated report has no
			// custom_time_range.
			{
				Config: testAccReportCustomTimeRangeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_report.ctr_clear",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.ctr_clear",
						tfjsonpath.New("config").AtMapKey("custom_time_range"),
						knownvalue.Null()),
				},
			},
			// Step 3: drift check after replacement.
			{
				Config: testAccReportCustomTimeRangeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithCustomTimeRange(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "ctr_clear" {
    name = "test-ctr-clear-%d"
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
        time_range = {
            mode = "custom"
            unit = "day"
        }
        custom_time_range = {
            from = "2024-01-01T00:00:00Z"
            to   = "2024-02-01T00:00:00Z"
        }
    }
}
`, i)
}

func testAccReportCustomTimeRangeCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "ctr_clear" {
    name = "test-ctr-clear-%d"
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
        time_range = {
            mode   = "last"
            amount = 3
            unit   = "month"
        }
    }
}
`, i)
}

// TestAccReport_DisplaySettings_NotClearable proves config.display_settings
// cannot be cleared: removing it is silently preserved (idempotent, no drift) —
// the prior value sticks in state.
func TestAccReport_DisplaySettings_NotClearable(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.ds_test",
		tfjsonpath.New("config").AtMapKey("display_settings").AtMapKey("number_scale"),
		knownvalue.StringExact("millions"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH display_settings.
			{
				Config:            testAccReportWithDisplaySettings(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: omit display_settings. The API silently preserves it, so the
			// removal applies idempotently (no error) and the value stays in state.
			{
				Config:            testAccReportDisplaySettingsCleared(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 3: drift check — the retained value produces no diff, confirming
			// removal is silently ignored rather than causing a permadiff.
			{
				Config: testAccReportDisplaySettingsCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportDisplaySettingsCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "ds_test" {
    name        = "test-display-settings-%d"
    description = "Report testing display_settings block"
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

// TestAccReport_Metric_NotClearable proves config.metric (the deprecated singular
// metric) cannot be cleared to null: it is a computed mirror of metrics[0].
// Setting metric then switching to a metrics list drops the block from config, but
// the API keeps metric populated (now mirroring metrics[0]="usage"). The removal
// applies idempotently — the mirror does not drift (no permadiff).
// TestAccReport_Metric_UnconfiguredMirrorNotStored pins the handling of the
// metric/metrics mirror pair. The API derives both from one field and always
// returns both populated, but their type/value leaves are Required
// (non-computed), so storing the echo for the mirror the practitioner did not
// configure makes Terraform Core propose null for it on the next plan and
// cascades into a permanent whole-resource diff. The provider therefore keeps
// the unconfigured mirror out of state entirely (plan modifier + state-aware
// Read); see useNullForUnconfiguredMetricMirror.
//
// This is a state-tracking decision, not a clearing one: the API keeps deriving
// `metric` server-side either way — the provider just stops recording it.
func TestAccReport_Metric_UnconfiguredMirrorNotStored(t *testing.T) {
	n := acctest.RandInt()

	// Step 1: metric is configured, so it is tracked; metrics is not configured,
	// so its mirror stays out of state as an empty list.
	metricCost := statecheck.ExpectKnownValue(
		"doit_report.metric_clear",
		tfjsonpath.New("config").AtMapKey("metric").AtMapKey("value"),
		knownvalue.StringExact("cost"))
	metricsEmpty := statecheck.ExpectKnownValue(
		"doit_report.metric_clear",
		tfjsonpath.New("config").AtMapKey("metrics"),
		knownvalue.ListExact([]knownvalue.Check{}))
	// Step 2: after switching to metrics=[usage] and dropping the metric block,
	// the roles swap — metrics is tracked and the metric mirror drops to null.
	metricNull := statecheck.ExpectKnownValue(
		"doit_report.metric_clear",
		tfjsonpath.New("config").AtMapKey("metric"),
		knownvalue.Null())
	metricsUsage := statecheck.ExpectKnownValue(
		"doit_report.metric_clear",
		tfjsonpath.New("config").AtMapKey("metrics").AtSliceIndex(0).AtMapKey("value"),
		knownvalue.StringExact("usage"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH the singular metric.
			{
				Config:            testAccReportWithSingularMetric(n),
				ConfigStateChecks: []statecheck.StateCheck{metricCost, metricsEmpty},
			},
			// Step 2: switch to the metrics list, dropping metric.
			{
				Config:            testAccReportMetricCleared(n),
				ConfigStateChecks: []statecheck.StateCheck{metricNull, metricsUsage},
			},
			// Step 3: drift check — this is the regression guard for the phantom
			// diff; without the mirror handling this plan never converges.
			{
				Config: testAccReportMetricCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSingularMetric(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "metric_clear" {
    name = "test-metric-clear-%d"
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

func testAccReportMetricCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "metric_clear" {
    name = "test-metric-clear-%d"
    config = {
        metrics = [
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

// TestAccReport_MetricFilter_RemovalForcesReplace proves config.metric_filter (and
// its required nested config.metric_filter.metric) cannot be cleared in place — an
// in-place update would send an empty metricFilter and the API would reject it
// (400) — so ModifyPlan forces a destroy+create (Category C) when it is removed.
func TestAccReport_MetricFilter_RemovalForcesReplace(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.mf_clear",
		tfjsonpath.New("config").AtMapKey("metric_filter").AtMapKey("operator"),
		knownvalue.StringExact("gt"))
	presentNested := statecheck.ExpectKnownValue(
		"doit_report.mf_clear",
		tfjsonpath.New("config").AtMapKey("metric_filter").AtMapKey("metric").AtMapKey("value"),
		knownvalue.StringExact("cost"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH metric_filter (including its nested metric).
			{
				Config:            testAccReportWithMetricFilter(n),
				ConfigStateChecks: []statecheck.StateCheck{present, presentNested},
			},
			// Step 2: omit metric_filter — removal forces a replacement, and the
			// recreated report has no metric_filter.
			{
				Config: testAccReportMetricFilterCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"doit_report.mf_clear",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"doit_report.mf_clear",
						tfjsonpath.New("config").AtMapKey("metric_filter"),
						knownvalue.Null()),
				},
			},
			// Step 3: drift check after replacement.
			{
				Config: testAccReportMetricFilterCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithMetricFilter(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "mf_clear" {
    name = "test-mf-clear-%d"
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
            operator = "gt"
            values   = [100]
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

func testAccReportMetricFilterCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "mf_clear" {
    name = "test-mf-clear-%d"
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

// TestAccReport_SecondaryTimeRange_NotClearable proves config.secondary_time_range
// (and its nested config.secondary_time_range.custom_time_range) cannot be cleared:
// removing the block is silently preserved (idempotent, no drift) — the prior
// value sticks in state.
func TestAccReport_SecondaryTimeRange_NotClearable(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.str_clear",
		tfjsonpath.New("config").AtMapKey("secondary_time_range").AtMapKey("custom_time_range").AtMapKey("from"),
		knownvalue.StringExact("2023-01-01T00:00:00Z"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH a secondary time range (custom dates).
			{
				Config:            testAccReportWithSecondaryCustom(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: omit secondary_time_range. The API silently preserves it, so the
			// removal applies idempotently (no error) and the value stays in state.
			{
				Config:            testAccReportSecondaryCleared(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 3: drift check — the retained value produces no diff, confirming
			// removal is silently ignored rather than causing a permadiff.
			{
				Config: testAccReportSecondaryCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithSecondaryCustom(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "str_clear" {
    name = "test-str-clear-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        time_range = {
            mode = "custom"
            unit = "day"
        }
        custom_time_range = {
            from = "2024-01-01T00:00:00Z"
            to   = "2024-12-31T23:59:59Z"
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

func testAccReportSecondaryCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "str_clear" {
    name = "test-str-clear-%d"
    config = {
        metric = {
            type  = "basic"
            value = "cost"
        }
        aggregation   = "total"
        time_interval = "month"
        time_range = {
            mode = "custom"
            unit = "day"
        }
        custom_time_range = {
            from = "2024-01-01T00:00:00Z"
            to   = "2024-12-31T23:59:59Z"
        }
        data_source    = "billing"
        display_values = "actuals_only"
        currency       = "USD"
        layout         = "table"
    }
}
`, i)
}

// TestAccReport_TimeRange_NotClearable proves config.time_range cannot be cleared:
// removing the explicit block is silently preserved (idempotent, no drift) — the
// stored range sticks in state.
func TestAccReport_TimeRange_NotClearable(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.tr_clear",
		tfjsonpath.New("config").AtMapKey("time_range").AtMapKey("amount"),
		knownvalue.Int64Exact(3))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH an explicit relative time range.
			{
				Config:            testAccReportWithExplicitTimeRange(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: omit time_range. The API silently preserves the stored range, so
			// the removal applies idempotently (no error) and the value stays in state.
			{
				Config:            testAccReportTimeRangeCleared(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 3: drift check — the retained value produces no diff, confirming
			// removal is silently ignored rather than causing a permadiff.
			{
				Config: testAccReportTimeRangeCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithExplicitTimeRange(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "tr_clear" {
    name = "test-tr-clear-%d"
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
        time_range = {
            mode   = "last"
            amount = 3
            unit   = "month"
        }
    }
}
`, i)
}

func testAccReportTimeRangeCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "tr_clear" {
    name = "test-tr-clear-%d"
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

// TestAccReport_Config_NotClearable proves the top-level config wrapper cannot be
// cleared: removing the entire config block is silently preserved (idempotent, no
// drift) — the API PATCH-merge keeps the stored config and Terraform copies the
// prior state. This documents the Category B classification of the top-level
// config object (clearableattr issue #233).
func TestAccReport_Config_NotClearable(t *testing.T) {
	n := acctest.RandInt()

	present := statecheck.ExpectKnownValue(
		"doit_report.cfg_clear",
		tfjsonpath.New("config").AtMapKey("aggregation"),
		knownvalue.StringExact("total"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			// Step 1: create WITH config.
			{
				Config:            testAccReportWithConfigForClear(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 2: omit the entire config block. The API silently preserves it, so
			// the removal applies idempotently (no error) and config stays in state.
			{
				Config:            testAccReportConfigCleared(n),
				ConfigStateChecks: []statecheck.StateCheck{present},
			},
			// Step 3: drift check — the retained config produces no diff, confirming
			// removal is silently ignored rather than causing a permadiff.
			{
				Config: testAccReportConfigCleared(n),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportWithConfigForClear(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "cfg_clear" {
    name = "test-cfg-clear-%d"
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

func testAccReportConfigCleared(i int) string {
	return fmt.Sprintf(`
resource "doit_report" "cfg_clear" {
    name = "test-cfg-clear-%d"
}
`, i)
}
