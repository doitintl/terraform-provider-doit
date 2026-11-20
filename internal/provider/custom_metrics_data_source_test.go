package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccCustomMetricsDataSourceMetrics returns two single allocations and
// three custom metrics named <prefix>-1 to <prefix>-3. Only the first has
// display precision, so the data source maps both a set and a null decimals.
// Data sources in extra read after all three exist.
func testAccCustomMetricsDataSourceMetrics(prefix, extra string) string {
	return fmt.Sprintf(`
resource "doit_allocation" "a" {
  name        = "%[1]s-a"
  description = "custom metrics data source test allocation"
  rule = {
    formula = "A"
    components = [
      { key = "country", mode = "is", type = "fixed", values = ["JP"] },
    ]
  }
}

resource "doit_allocation" "b" {
  name        = "%[1]s-b"
  description = "custom metrics data source test allocation"
  rule = {
    formula = "A"
    components = [
      { key = "country", mode = "is", type = "fixed", values = ["DE"] },
    ]
  }
}

resource "doit_custom_metric" "m1" {
  name        = "%[1]s-1"
  description = "first"
  formula     = "A / B"
  unit_type   = "percentage"
  decimals    = { min = 0, max = 2 }
  variables = [
    { metric = { type = "basic", value = "cost" }, allocation_id = doit_allocation.a.id },
    { metric = { type = "basic", value = "usage" }, allocation_id = doit_allocation.b.id },
  ]
}

resource "doit_custom_metric" "m2" {
  name    = "%[1]s-2"
  formula = "A"
  variables = [
    { metric = { type = "basic", value = "cost" }, allocation_id = doit_allocation.a.id },
  ]
}

resource "doit_custom_metric" "m3" {
  name    = "%[1]s-3"
  formula = "A + B"
  variables = [
    { metric = { type = "basic", value = "cost" }, allocation_id = doit_allocation.a.id },
    { metric = { type = "basic", value = "cost" }, allocation_id = doit_allocation.b.id },
  ]
}
%[2]s`, prefix, extra)
}

const testCustomMetricsDependsOn = `depends_on = [doit_custom_metric.m1, doit_custom_metric.m2, doit_custom_metric.m3]`

func testAccCustomMetricsNames(prefix string) knownvalue.Check {
	return knownvalue.ListExact([]knownvalue.Check{
		knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact(prefix + "-1")}),
		knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact(prefix + "-2")}),
		knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact(prefix + "-3")}),
	})
}

// TestAccCustomMetricsDataSource_Basic lists three metrics sorted by name and
// checks the mapped attributes, then that a second apply plans nothing.
func TestAccCustomMetricsDataSource_Basic(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tfacc-cms")
	const addr = "data.doit_custom_metrics.sorted"

	config := testAccCustomMetricsDataSourceMetrics(prefix, fmt.Sprintf(`
data "doit_custom_metrics" "sorted" {
  name_contains = %q
  sort_by       = "name"
  sort_order    = "asc"
  %s
}
`, prefix, testCustomMetricsDependsOn))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("row_count"), knownvalue.Int64Exact(3)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("page_token"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("max_results"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("items"), testAccCustomMetricsNames(prefix)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("items").AtSliceIndex(0), knownvalue.ObjectPartial(map[string]knownvalue.Check{
						"id":          knownvalue.NotNull(),
						"etag":        knownvalue.NotNull(),
						"description": knownvalue.StringExact("first"),
						"owner":       knownvalue.NotNull(),
						"formula":     knownvalue.StringExact("A / B"),
						"unit_type":   knownvalue.StringExact("percentage"),
						"create_time": knownvalue.NotNull(),
						"update_time": knownvalue.NotNull(),
						"decimals": knownvalue.ObjectExact(map[string]knownvalue.Check{
							"min": knownvalue.Int64Exact(0),
							"max": knownvalue.Int64Exact(2),
						}),
						"variables": knownvalue.ListExact([]knownvalue.Check{
							knownVariable("A", "basic", "cost"),
							knownVariable("B", "basic", "usage"),
						}),
					})),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("items").AtSliceIndex(1), knownvalue.ObjectPartial(map[string]knownvalue.Check{
						"decimals":  knownvalue.Null(),
						"variables": knownvalue.ListExact([]knownvalue.Check{knownVariable("A", "basic", "cost")}),
					})),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetricsDataSource_Filter combines name_contains, a name filter
// and an owner filter.
func TestAccCustomMetricsDataSource_Filter(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tfacc-cmf")

	config := testAccCustomMetricsDataSourceMetrics(prefix, fmt.Sprintf(`
data "doit_custom_metrics" "by_name" {
  filter = "name:%[1]s-2"
  %[2]s
}

data "doit_custom_metrics" "by_owner" {
  name_contains = %[1]q
  filter        = "owner:${urlencode(doit_custom_metric.m1.owner)}"
  %[2]s
}
`, prefix, testCustomMetricsDependsOn))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_custom_metrics.by_name", tfjsonpath.New("row_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.by_name", tfjsonpath.New("items"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact(prefix + "-2")}),
					})),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.by_owner", tfjsonpath.New("row_count"), knownvalue.Int64Exact(3)),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetricsDataSource_Pagination covers a single page, the next page
// from its token, and auto-pagination from a token.
func TestAccCustomMetricsDataSource_Pagination(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tfacc-cmp")

	config := testAccCustomMetricsDataSourceMetrics(prefix, fmt.Sprintf(`
data "doit_custom_metrics" "first" {
  name_contains = %[1]q
  sort_by       = "name"
  sort_order    = "asc"
  max_results   = 2
  %[2]s
}

data "doit_custom_metrics" "second" {
  name_contains = %[1]q
  sort_by       = "name"
  sort_order    = "asc"
  max_results   = 2
  page_token    = data.doit_custom_metrics.first.page_token
}

data "doit_custom_metrics" "rest" {
  name_contains = %[1]q
  sort_by       = "name"
  sort_order    = "asc"
  page_token    = data.doit_custom_metrics.first.page_token
}
`, prefix, testCustomMetricsDependsOn))

	nameCheck := func(suffix string) knownvalue.Check {
		return knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact(prefix + suffix)})
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_custom_metrics.first", tfjsonpath.New("items"),
						knownvalue.ListExact([]knownvalue.Check{nameCheck("-1"), nameCheck("-2")})),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.first", tfjsonpath.New("page_token"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.first", tfjsonpath.New("row_count"), knownvalue.Int64Exact(3)),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.first", tfjsonpath.New("max_results"), knownvalue.Int64Exact(2)),

					statecheck.ExpectKnownValue("data.doit_custom_metrics.second", tfjsonpath.New("items"),
						knownvalue.ListExact([]knownvalue.Check{nameCheck("-3")})),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.second", tfjsonpath.New("page_token"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.second", tfjsonpath.New("row_count"), knownvalue.Int64Exact(3)),

					// A page_token alone auto-paginates from the token and leaves max_results null.
					statecheck.ExpectKnownValue("data.doit_custom_metrics.rest", tfjsonpath.New("items"),
						knownvalue.ListExact([]knownvalue.Check{nameCheck("-3")})),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.rest", tfjsonpath.New("max_results"), knownvalue.Null()),
					statecheck.ExpectKnownValue("data.doit_custom_metrics.rest", tfjsonpath.New("row_count"), knownvalue.Int64Exact(3)),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetricsDataSource_Empty checks that no match is an empty list,
// not null.
func TestAccCustomMetricsDataSource_Empty(t *testing.T) {
	const addr = "data.doit_custom_metrics.none"
	config := fmt.Sprintf(`
data "doit_custom_metrics" "none" {
  name_contains = %q
}
`, acctest.RandomWithPrefix("tfacc-cm-no-such-metric"))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("items"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("row_count"), knownvalue.Int64Exact(0)),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetricsDataSource_UnknownInput reads with a name_contains that
// is unknown until the metric exists.
func TestAccCustomMetricsDataSource_UnknownInput(t *testing.T) {
	rName := acctest.RandomWithPrefix("tfacc-cmu")
	const addr = "data.doit_custom_metrics.this"

	config := testAccCustomMetricConfig(rName, "A / B",
		[]customMetricVariable{testCustomMetricCostA, testCustomMetricUsageB}, "") + `
data "doit_custom_metrics" "this" {
  name_contains = doit_custom_metric.this.name
}
`

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("row_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("items").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact(rName)),
				},
			},
			expectEmptyPlanStep(config),
		},
	})
}

// TestAccCustomMetricsDataSource_InvalidFilterKey checks that the API's
// rejection of an unsupported filter key reaches the user.
func TestAccCustomMetricsDataSource_InvalidFilterKey(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_custom_metrics" "bad" {
  filter = "unitType:number"
}
`,
				ExpectError: regexp.MustCompile(`filter_key_not_supported`),
			},
		},
	})
}
