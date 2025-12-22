package provider_test

import (
	"fmt"
	"testing"

	"math/rand/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccReport(t *testing.T) {
	n := rand.Int()

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
	n := rand.Int()

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
	n := rand.Int()

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
